package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/BGriffin63/reelping/internal/auth"
	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/security"
)

func (a *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Welcome to ReelPing", "setup")
	vd.Data = map[string]any{
		"Presets":     config.Presets(),
		"PresetOrder": config.PresetOrder,
		"TimeZones":   commonTimeZones,
	}
	a.render(w, "setup", vd)
}

func (a *App) handleSetupPost(w http.ResponseWriter, r *http.Request) {
	if has, _ := a.store.HasAdmin(); has {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request.", http.StatusBadRequest)
		return
	}

	fail := func(msg string) {
		vd := a.newViewData(w, r, "Welcome to ReelPing", "setup")
		vd.Flash = &flash{Kind: "err", Message: msg}
		vd.Data = map[string]any{"Presets": config.Presets(), "PresetOrder": config.PresetOrder, "TimeZones": commonTimeZones}
		w.WriteHeader(http.StatusBadRequest)
		a.render(w, "setup", vd)
	}

	// Step 1: administrator.
	username := security.CleanText(r.FormValue("username"), 64, true)
	if username == "" {
		fail("An administrator username is required.")
		return
	}
	password := r.FormValue("password")
	if password != r.FormValue("confirm_password") {
		fail("The two passwords do not match.")
		return
	}
	if ps := auth.CheckPassword(password); !ps.Acceptable {
		fail("Password is not strong enough: " + ps.Reason)
		return
	}

	cfg := config.Default()

	// Step 4: monitoring preset first (so custom values can override).
	preset := r.FormValue("preset")
	cfg.Monitoring = config.ApplyPreset(cfg.Monitoring, preset)
	if preset == "custom" {
		applyCustomMonitoring(&cfg, r)
	}
	if tz := r.FormValue("time_zone"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			cfg.General.TimeZone = tz
		}
	}

	// Step 2: Plex.
	cfg.Plex.DisplayName = firstNonEmpty(security.CleanText(r.FormValue("plex_display_name"), 64, true), "Plex")
	cfg.Plex.VerifyTLS = r.FormValue("verify_tls") != ""
	if to, err := strconv.Atoi(r.FormValue("plex_timeout")); err == nil && to >= 1 && to < cfg.Monitoring.CheckIntervalSeconds {
		cfg.Plex.TimeoutSeconds = to
	}
	plexURL := r.FormValue("plex_url")
	if plexURL != "" {
		normalized, err := security.ValidatePlexBaseURL(plexURL)
		if err != nil {
			fail("Plex URL is invalid: " + err.Error())
			return
		}
		cfg.Plex.BaseURL = normalized
	}
	if tok := r.FormValue("plex_token"); tok != "" {
		cfg.Plex.PlexToken = tok
		cfg.Plex.SessionIntegration = true
	}
	if mid := r.FormValue("plex_machine_id"); mid != "" {
		if v, err := security.ValidateMachineIdentifier(mid); err == nil {
			cfg.Plex.ExpectedMachineID = v
		}
	}

	// Step 3: Discord.
	if wh := r.FormValue("discord_webhook"); wh != "" {
		normalized, err := security.ValidateDiscordWebhookURL(wh)
		if err != nil {
			fail("Discord webhook URL is invalid: " + err.Error())
			return
		}
		cfg.Discord.WebhookURL = normalized
	}
	cfg.Discord.UsernameOverride = security.CleanText(r.FormValue("discord_username"), 80, true)

	// Step 5: finish — monitoring enable.
	wantMonitoring := r.FormValue("enable_monitoring") != ""
	if wantMonitoring && cfg.Plex.BaseURL == "" {
		fail("To enable monitoring, please provide a Plex URL (or leave monitoring off for now).")
		return
	}
	cfg.General.MonitoringOn = wantMonitoring
	cfg.SetupComplete = true

	if err := cfg.Validate(); err != nil {
		fail("Configuration is invalid: " + err.Error())
		return
	}

	// Persist admin + config.
	hash, err := auth.HashPassword(password)
	if err != nil {
		fail("Could not secure the password. Please try again.")
		return
	}
	now := time.Now().UTC()
	if err := a.store.SaveAdmin(model.Admin{Username: username, PasswordHash: hash, CreatedAt: now, UpdatedAt: now}); err != nil {
		fail("Could not save the administrator account.")
		return
	}
	if err := a.store.SaveConfig(cfg); err != nil {
		fail("Could not save configuration.")
		return
	}

	// Log the new admin in.
	sess, err := a.sessions.Create(w, r, username)
	if err == nil {
		sess.IPTag = a.clientIPTag(r)
		_ = a.store.PutSession(sess)
	}
	a.audit(r, "setup_complete", "administrator created")
	a.setFlash(w, "ok", "Setup complete. Welcome to ReelPing!")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func applyCustomMonitoring(cfg *config.Config, r *http.Request) {
	m := &cfg.Monitoring
	m.Preset = "custom"
	if v, err := strconv.Atoi(r.FormValue("check_interval")); err == nil {
		m.CheckIntervalSeconds = v
	}
	if v, err := strconv.Atoi(r.FormValue("request_timeout")); err == nil {
		m.TimeoutSeconds = v
	}
	if v, err := strconv.Atoi(r.FormValue("failure_threshold")); err == nil {
		m.FailureThreshold = v
	}
	if v, err := strconv.Atoi(r.FormValue("recovery_threshold")); err == nil {
		m.RecoveryThreshold = v
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

var commonTimeZones = []string{
	"UTC", "America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
	"America/Toronto", "America/Sao_Paulo", "Europe/London", "Europe/Paris", "Europe/Berlin",
	"Europe/Madrid", "Europe/Amsterdam", "Europe/Stockholm", "Europe/Moscow", "Asia/Dubai",
	"Asia/Kolkata", "Asia/Singapore", "Asia/Tokyo", "Asia/Shanghai", "Australia/Sydney", "Pacific/Auckland",
}
