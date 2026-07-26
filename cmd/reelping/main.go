// Command reelping is the ReelPing server: a lightweight Plex status monitor
// and Discord notifier with an authenticated web UI.
//
// Subcommands / flags:
//
//	reelping                 run the server
//	reelping -healthcheck    probe the local /healthz endpoint (for Docker)
//	reelping -reset-admin    clear the administrator account (local recovery)
//	reelping -version        print version and exit
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	// Embed the time-zone database so LoadLocation works in a minimal
	// (scratch/distroless) image without system zoneinfo.
	_ "time/tzdata"

	"github.com/BGriffin63/reelping/internal/auth"
	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/monitoring"
	"github.com/BGriffin63/reelping/internal/notify"
	"github.com/BGriffin63/reelping/internal/plex"
	"github.com/BGriffin63/reelping/internal/storage"
	"github.com/BGriffin63/reelping/internal/version"
	"github.com/BGriffin63/reelping/internal/web"
)

func main() {
	var (
		healthcheck = flag.Bool("healthcheck", false, "probe the local health endpoint and exit")
		resetAdmin  = flag.Bool("reset-admin", false, "clear the administrator account for local recovery, then exit")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	addr := envDefault("RP_ADDR", ":8787")
	configDir := envDefault("RP_CONFIG_DIR", "/config")
	dbPath := filepath.Join(configDir, "reelping.db")

	if *healthcheck {
		os.Exit(runHealthcheck(addr))
	}

	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	store, err := storage.Open(dbPath)
	if err != nil {
		logger.Fatalf("could not open database at %s: %v", dbPath, err)
	}
	defer store.Close()

	if *resetAdmin {
		if err := store.DeleteAdmin(); err != nil {
			logger.Fatalf("reset-admin failed: %v", err)
		}
		logger.Printf("administrator account cleared; the first-run wizard will run on next start")
		return
	}

	if err := run(addr, store, logger); err != nil {
		logger.Fatalf("server error: %v", err)
	}
}

func run(addr string, store *storage.Store, logger *log.Logger) error {
	cfg, _ := store.GetConfig()

	sessions := auth.NewManager(
		store,
		time.Duration(orDefault(cfg.Security.SessionIdleMinutes, 120))*time.Minute,
		time.Duration(orDefault(cfg.Security.SessionAbsoluteHours, 168))*time.Hour,
	)
	notifier := notify.New(store, logger.Printf)

	check := func(ctx context.Context, cfg config.Config) plex.CheckResult {
		if cfg.Plex.BaseURL == "" {
			return plex.CheckResult{Classification: plex.InvalidResponse, Detail: "No Plex URL is configured."}
		}
		client, err := plex.New(plex.Options{
			BaseURL:              cfg.Plex.BaseURL,
			Token:                cfg.Plex.PlexToken,
			ExpectedMachineID:    cfg.Plex.ExpectedMachineID,
			VerifyTLS:            cfg.Plex.VerifyTLS,
			Timeout:              time.Duration(cfg.Plex.TimeoutSeconds) * time.Second,
			SupplementalHostDiag: cfg.Monitoring.SupplementalHostDiag,
			FetchSessions:        cfg.Plex.HasToken() && cfg.Plex.SessionIntegration,
		})
		if err != nil {
			return plex.CheckResult{Classification: plex.UnknownFailure, Detail: "Invalid Plex configuration."}
		}
		return client.Check(ctx)
	}

	worker := monitoring.NewWorker(store, notifier, check, logger.Printf)

	app, err := web.NewApp(web.Deps{
		Store:    store,
		Sessions: sessions,
		Notifier: notifier,
		Worker:   worker,
		Logger:   logger,
	})
	if err != nil {
		return fmt.Errorf("build web app: %w", err)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Background workers.
	go worker.Run(ctx)
	go backgroundMaintenance(ctx, store, sessions, logger)

	go func() {
		<-ctx.Done()
		logger.Printf("shutdown signal received; draining...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Printf("%s listening on %s", version.String(), addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	logger.Printf("stopped cleanly")
	return nil
}

// backgroundMaintenance runs periodic retention, session purging, and rate
// limiter sweeps.
func backgroundMaintenance(ctx context.Context, store *storage.Store, sessions *auth.Manager, logger *log.Logger) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	sweep := func() {
		cfg, _ := store.GetConfig()
		now := time.Now().UTC()
		if _, err := store.ApplyRetention(cfg.General.RetentionDays, now); err != nil {
			logger.Printf("retention sweep error: %v", err)
		}
		if _, err := store.PurgeExpiredSessions(now); err != nil {
			logger.Printf("session purge error: %v", err)
		}
	}
	sweep()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep()
		}
	}
}

// runHealthcheck probes the local /healthz endpoint. It returns 0 if healthy.
func runHealthcheck(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "8787"
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
