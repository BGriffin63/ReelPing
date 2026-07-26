package web

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BGriffin63/reelping/internal/auth"
	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/monitoring"
	"github.com/BGriffin63/reelping/internal/notify"
	"github.com/BGriffin63/reelping/internal/plex"
	"github.com/BGriffin63/reelping/internal/storage"
)

func newTestApp(t *testing.T) (*App, *storage.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "reelping.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	logger := log.New(io.Discard, "", 0)
	sessions := auth.NewManager(store, 2*time.Hour, 168*time.Hour)
	notifier := notify.New(store, logger.Printf)
	check := func(ctx context.Context, cfg config.Config) plex.CheckResult {
		return plex.CheckResult{OK: true, Classification: plex.Online, LatencyMillis: 10, Detail: "ok"}
	}
	worker := monitoring.NewWorker(store, notifier, check, logger.Printf)

	app, err := NewApp(Deps{Store: store, Sessions: sessions, Notifier: notifier, Worker: worker, Logger: logger})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	return app, store
}

func TestHealthzAlwaysOK(t *testing.T) {
	app, _ := newTestApp(t)
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz status %d", resp.StatusCode)
	}
}

func TestFirstRunRedirectsToSetup(t *testing.T) {
	app, _ := newTestApp(t)
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()

	client := noRedirectClient()
	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/setup" {
		t.Fatalf("expected redirect to /setup, got %q", loc)
	}
}

func TestSetupPageRenders(t *testing.T) {
	app, _ := newTestApp(t)
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	body := getBody(t, srv.URL+"/setup")
	if !strings.Contains(body, "Administrator account") {
		t.Fatalf("setup page missing expected content")
	}
}

// TestFullSetupFlowAndAllPages runs the wizard and then loads every page,
// proving all templates parse and render without error.
func TestFullSetupFlowAndAllPages(t *testing.T) {
	app, _ := newTestApp(t)
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Complete setup.
	form := url.Values{
		"username":          {"admin"},
		"password":          {"a-strong-passphrase-123"},
		"confirm_password":  {"a-strong-passphrase-123"},
		"plex_display_name": {"Plex"},
		"plex_url":          {"http://192.168.1.10:32400"},
		"preset":            {"balanced"},
		"time_zone":         {"UTC"},
		"enable_monitoring": {"on"},
	}
	resp, err := client.PostForm(srv.URL+"/setup", form)
	if err != nil {
		t.Fatalf("setup post: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("setup returned %d", resp.StatusCode)
	}

	// Now all authenticated pages must render.
	pages := []string{"/", "/maintenance", "/announcements", "/incidents", "/notifications", "/audit", "/settings", "/diagnostics"}
	for _, p := range pages {
		r, err := client.Get(srv.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatalf("GET %s returned %d: %s", p, r.StatusCode, string(b)[:min(len(b), 300)])
		}
		if !strings.Contains(string(b), "ReelPing") {
			t.Fatalf("GET %s did not render a ReelPing page", p)
		}
	}
}

func TestLoginLogoutFlow(t *testing.T) {
	app, store := newTestApp(t)
	// Create admin directly.
	hash, _ := auth.HashPassword("a-strong-passphrase-123")
	_ = store.SaveAdmin(mkAdmin("admin", hash))
	cfg := config.Default()
	cfg.SetupComplete = true
	_ = store.SaveConfig(cfg)

	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Wrong password rejected.
	resp, _ := client.PostForm(srv.URL+"/login", url.Values{"username": {"admin"}, "password": {"wrong"}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login should be 401, got %d", resp.StatusCode)
	}

	// Correct login.
	resp, _ = client.PostForm(srv.URL+"/login", url.Values{"username": {"admin"}, "password": {"a-strong-passphrase-123"}})
	resp.Body.Close()

	// Dashboard now accessible.
	r, _ := client.Get(srv.URL + "/")
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || !strings.Contains(string(b), "Overview") {
		t.Fatalf("expected dashboard after login, got %d", r.StatusCode)
	}
}

func TestCSRFRequiredForActions(t *testing.T) {
	app, store := newTestApp(t)
	hash, _ := auth.HashPassword("a-strong-passphrase-123")
	_ = store.SaveAdmin(mkAdmin("admin", hash))
	cfg := config.Default()
	cfg.SetupComplete = true
	_ = store.SaveConfig(cfg)

	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, _ := client.PostForm(srv.URL+"/login", url.Values{"username": {"admin"}, "password": {"a-strong-passphrase-123"}})
	resp.Body.Close()

	// POST without CSRF token must be rejected.
	resp, _ = client.PostForm(srv.URL+"/actions/announce", url.Values{"title": {"x"}, "message": {"y"}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected CSRF rejection (403), got %d", resp.StatusCode)
	}
}

func TestDiagnosticsDownloadHasNoSecrets(t *testing.T) {
	app, store := newTestApp(t)
	hash, _ := auth.HashPassword("a-strong-passphrase-123")
	_ = store.SaveAdmin(mkAdmin("admin", hash))
	cfg := config.Default()
	cfg.SetupComplete = true
	cfg.Plex.PlexToken = "SECRET-PLEX-TOKEN-XYZ"
	cfg.Discord.WebhookURL = "https://discord.com/api/webhooks/123456789012345678/SECRET-WEBHOOK-TOKEN"
	_ = store.SaveConfig(cfg)

	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, _ := client.PostForm(srv.URL+"/login", url.Values{"username": {"admin"}, "password": {"a-strong-passphrase-123"}})
	resp.Body.Close()

	r, _ := client.Get(srv.URL + "/diagnostics/download")
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	s := string(b)
	if strings.Contains(s, "SECRET-PLEX-TOKEN-XYZ") {
		t.Fatalf("diagnostics leaked the Plex token")
	}
	if strings.Contains(s, "SECRET-WEBHOOK-TOKEN") {
		t.Fatalf("diagnostics leaked the Discord webhook")
	}
	if strings.Contains(s, hash) {
		t.Fatalf("diagnostics leaked the password hash")
	}
}

// TestSetupPlexTestReadsWizardFieldName guards against the field-name mismatch
// where the setup wizard posts "plex_url" but the handler only read "base_url".
func TestSetupPlexTestReadsWizardFieldName(t *testing.T) {
	// Fake Plex that answers /identity.
	plexSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			_, _ = w.Write([]byte(`<MediaContainer machineIdentifier="ABC123" version="1.40"/>`))
			return
		}
		w.WriteHeader(404)
	}))
	defer plexSrv.Close()

	app, _ := newTestApp(t) // no admin yet -> setup endpoints are open
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()

	resp, err := http.PostForm(srv.URL+"/setup/test/plex", url.Values{"plex_url": {plexSrv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	s := string(b)
	if strings.Contains(s, "value is required") {
		t.Fatalf("setup Plex test did not read the plex_url field: %s", s)
	}
	if !strings.Contains(s, `"ok":true`) {
		t.Fatalf("expected a successful test against the fake Plex, got: %s", s)
	}
}

func mkAdmin(username, hash string) model.Admin {
	now := time.Now().UTC()
	return model.Admin{Username: username, PasswordHash: hash, CreatedAt: now, UpdatedAt: now}
}

func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func getBody(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
