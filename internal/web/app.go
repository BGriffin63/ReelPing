// Package web implements ReelPing's authenticated server-rendered web
// interface: the first-run wizard, dashboard, maintenance/announcement actions,
// history views, settings, and diagnostics.
package web

import (
	"context"
	"encoding/base64"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/BGriffin63/reelping/internal/auth"
	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/monitoring"
	"github.com/BGriffin63/reelping/internal/notify"
	"github.com/BGriffin63/reelping/internal/plex"
	"github.com/BGriffin63/reelping/internal/security"
	"github.com/BGriffin63/reelping/internal/storage"
	"github.com/BGriffin63/reelping/internal/version"
	webassets "github.com/BGriffin63/reelping/web"
)

// App holds the web application dependencies.
type App struct {
	store        *storage.Store
	sessions     *auth.Manager
	loginLimiter *auth.RateLimiter
	announceLim  *auth.RateLimiter
	notifier     *notify.Service
	worker       *monitoring.Worker
	tmpl         *template.Template
	logger       *log.Logger
	build        version.Info

	trustedProxies []string
}

// Deps bundles the App constructor dependencies.
type Deps struct {
	Store    *storage.Store
	Sessions *auth.Manager
	Notifier *notify.Service
	Worker   *monitoring.Worker
	Logger   *log.Logger
}

// NewApp builds the web application and parses templates.
func NewApp(d Deps) (*App, error) {
	cfg, _ := d.Store.GetConfig()
	tmpl, err := parseTemplates()
	if err != nil {
		return nil, err
	}
	a := &App{
		store:          d.Store,
		sessions:       d.Sessions,
		loginLimiter:   auth.NewRateLimiter(max(cfg.Security.LoginMaxAttempts, 5), time.Duration(max(cfg.Security.LoginWindowSeconds, 300))*time.Second),
		announceLim:    auth.NewRateLimiter(20, time.Minute),
		notifier:       d.Notifier,
		worker:         d.Worker,
		tmpl:           tmpl,
		logger:         d.Logger,
		build:          version.Get(),
		trustedProxies: cfg.Security.TrustedProxies,
	}
	return a, nil
}

func parseTemplates() (*template.Template, error) {
	funcs := template.FuncMap{
		"shortid": model.ShortID,
		"fdur":    monitoring.FormatDuration,
		"upper":   strings.ToUpper,
		"yesno": func(b bool) string {
			if b {
				return "Yes"
			}
			return "No"
		},
	}
	return template.New("").Funcs(funcs).ParseFS(webassets.Templates, "templates/*.html")
}

// Handler returns the root HTTP handler with all routes and middleware.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static assets (embedded).
	staticFS, _ := fs.Sub(webassets.Static, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheStatic(http.FileServer(http.FS(staticFS)))))

	// Health endpoints (unauthenticated; never reveal secrets or Plex state).
	mux.HandleFunc("GET /healthz", a.handleHealthz)

	// First-run setup wizard.
	mux.HandleFunc("GET /setup", a.handleSetup)
	mux.HandleFunc("POST /setup", a.handleSetupPost)
	// Connection tests available only during first-run (no session yet).
	mux.HandleFunc("POST /setup/test/plex", a.setupOnly(a.handleTestPlex))
	mux.HandleFunc("POST /setup/test/discord", a.setupOnly(a.handleTestDiscord))

	// Auth.
	mux.HandleFunc("GET /login", a.handleLoginForm)
	mux.HandleFunc("POST /login", a.handleLoginPost)
	mux.HandleFunc("GET /logout", a.requireAuthFunc(a.handleLogout))
	mux.HandleFunc("POST /logout", a.requireAuthFunc(a.handleLogout))

	// Authenticated pages.
	mux.HandleFunc("GET /{$}", a.requireAuthFunc(a.handleDashboard))
	mux.HandleFunc("GET /maintenance", a.requireAuthFunc(a.handleMaintenance))
	mux.HandleFunc("GET /announcements", a.requireAuthFunc(a.handleAnnouncements))
	mux.HandleFunc("GET /incidents", a.requireAuthFunc(a.handleIncidents))
	mux.HandleFunc("GET /incidents/{id}", a.requireAuthFunc(a.handleIncidentDetail))
	mux.HandleFunc("GET /notifications", a.requireAuthFunc(a.handleNotifications))
	mux.HandleFunc("GET /audit", a.requireAuthFunc(a.handleAudit))
	mux.HandleFunc("GET /settings", a.requireAuthFunc(a.handleSettings))
	mux.HandleFunc("GET /diagnostics", a.requireAuthFunc(a.handleDiagnostics))

	// Authenticated actions (CSRF-protected POSTs).
	mux.HandleFunc("POST /actions/monitoring", a.protected(a.handleToggleMonitoring))
	mux.HandleFunc("POST /actions/maintenance/schedule", a.protected(a.handleMaintenanceSchedule))
	mux.HandleFunc("POST /actions/maintenance/start", a.protected(a.handleMaintenanceStart))
	mux.HandleFunc("POST /actions/maintenance/offline", a.protected(a.handleGoingOffline))
	mux.HandleFunc("POST /actions/maintenance/delay", a.protected(a.handleMaintenanceDelay))
	mux.HandleFunc("POST /actions/maintenance/end", a.protected(a.handleMaintenanceEnd))
	mux.HandleFunc("POST /actions/restore", a.protected(a.handleServiceRestored))
	mux.HandleFunc("POST /actions/announce", a.protected(a.handleCustomAnnounce))

	mux.HandleFunc("POST /settings/plex", a.protected(a.handleSavePlex))
	mux.HandleFunc("POST /settings/discord", a.protected(a.handleSaveDiscord))
	mux.HandleFunc("POST /settings/monitoring", a.protected(a.handleSaveMonitoring))
	mux.HandleFunc("POST /settings/general", a.protected(a.handleSaveGeneral))
	mux.HandleFunc("POST /settings/security/password", a.protected(a.handleChangePassword))
	mux.HandleFunc("POST /settings/security/sessions", a.protected(a.handleSignOutOthers))
	mux.HandleFunc("POST /settings/data/clear", a.protected(a.handleClearHistory))

	// Async test endpoints (CSRF-protected; return JSON).
	mux.HandleFunc("POST /api/test/plex", a.protected(a.handleTestPlex))
	mux.HandleFunc("POST /api/test/discord", a.protected(a.handleTestDiscord))

	// Diagnostics/backup downloads.
	mux.HandleFunc("GET /diagnostics/download", a.requireAuthFunc(a.handleDiagnosticsDownload))
	mux.HandleFunc("GET /settings/data/backup", a.requireAuthFunc(a.handleBackupDownload))
	mux.HandleFunc("GET /incidents/export", a.requireAuthFunc(a.handleIncidentsExport))

	return a.baseMiddleware(mux)
}

// --- View data & rendering ---

type flash struct {
	Kind    string
	Message string
}

type viewData struct {
	Title    string
	Active   string
	Nonce    string
	CSRF     string
	Username string
	Cfg      config.Config
	Safe     config.SafeConfig
	State    model.MonitorState
	Last     plex.CheckResult
	Flash    *flash
	Build    version.Info
	Now      time.Time
	Idem     string
	Data     any
}

func (a *App) newViewData(w http.ResponseWriter, r *http.Request, title, active string) viewData {
	cfg, _ := a.store.GetConfig()
	sess, _ := a.currentSession(r)
	vd := viewData{
		Title:  title,
		Active: active,
		Nonce:  nonceFromContext(r.Context()),
		Cfg:    cfg,
		Safe:   cfg.Safe(security.RedactHint),
		Build:  a.build,
		Now:    time.Now().In(cfg.Location()),
		Flash:  a.readFlash(w, r),
		Idem:   model.NewID(),
	}
	if a.worker != nil {
		vd.State = a.worker.State()
		vd.Last = a.worker.LastResult()
	}
	vd.Username = sess.Username
	vd.CSRF = sess.CSRFToken
	return vd
}

func (a *App) render(w http.ResponseWriter, name string, vd viewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, name, vd); err != nil {
		a.logger.Printf("template %q error: %v", name, err)
		http.Error(w, "Internal error rendering page.", http.StatusInternalServerError)
	}
}

// --- viewData template methods ---

// FTime formats a time (time.Time or *time.Time) in the configured zone/format.
func (v viewData) FTime(t any) string {
	switch tt := t.(type) {
	case time.Time:
		return monitoring.FormatTime(v.Cfg, tt)
	case *time.Time:
		if tt == nil {
			return "—"
		}
		return monitoring.FormatTime(v.Cfg, *tt)
	default:
		return "—"
	}
}

// StatusText returns a human label for the current monitor state.
func (v viewData) StatusText() string { return stateLabel(v.State.State) }

// StatusBadgeClass maps state to a badge class.
func (v viewData) StatusBadgeClass() string {
	switch v.State.State {
	case monitoring.StateOnline, monitoring.StateMaintenanceOnline:
		return "ok"
	case monitoring.StateOffline, monitoring.StateMaintenanceOffline:
		return "bad"
	case monitoring.StateSuspect, monitoring.StateDegraded, monitoring.StateRecovering:
		return "warn"
	default:
		return "neutral"
	}
}

// StatusHeroClass maps state to a hero colour class defined in app.css.
func (v viewData) StatusHeroClass() string {
	switch v.State.State {
	case monitoring.StateOnline:
		return "s-online"
	case monitoring.StateOffline:
		return "s-offline"
	case monitoring.StateSuspect:
		return "s-suspect"
	case monitoring.StateRecovering:
		return "s-recovering"
	case monitoring.StateDegraded:
		return "s-degraded"
	case monitoring.StateMaintenanceOnline, monitoring.StateMaintenanceOffline:
		return "s-maintenance"
	default:
		return "s-unknown"
	}
}

func stateLabel(s string) string {
	switch s {
	case monitoring.StateOnline:
		return "Online"
	case monitoring.StateOffline:
		return "Offline"
	case monitoring.StateSuspect:
		return "Suspect"
	case monitoring.StateRecovering:
		return "Recovering"
	case monitoring.StateDegraded:
		return "Degraded"
	case monitoring.StateMaintenanceOnline:
		return "Maintenance (online)"
	case monitoring.StateMaintenanceOffline:
		return "Maintenance (offline)"
	case monitoring.StateInitializing:
		return "Starting up"
	case monitoring.StateDisabled:
		return "Monitoring off"
	default:
		return "Unknown"
	}
}

// --- flash cookie helpers ---

func (a *App) setFlash(w http.ResponseWriter, kind, msg string) {
	val := base64.RawURLEncoding.EncodeToString([]byte(kind + "\x1f" + msg))
	http.SetCookie(w, &http.Cookie{Name: "rp_flash", Value: val, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 30})
}

func (a *App) readFlash(w http.ResponseWriter, r *http.Request) *flash {
	c, err := r.Cookie("rp_flash")
	if err != nil || c.Value == "" {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: "rp_flash", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil
	}
	parts := strings.SplitN(string(raw), "\x1f", 2)
	if len(parts) != 2 {
		return nil
	}
	return &flash{Kind: parts[0], Message: parts[1]}
}

func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		h.ServeHTTP(w, r)
	})
}

// audit writes an audit event (best-effort; never records secrets).
func (a *App) audit(r *http.Request, action, detail string) {
	sess, _ := a.currentSession(r)
	_ = a.store.PutAudit(model.AuditEvent{
		ID:     model.NewID(),
		Time:   time.Now().UTC(),
		Action: action,
		Actor:  sess.Username,
		Detail: detail,
		IPTag:  a.clientIPTag(r),
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ = context.Background
