package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BGriffin63/reelping/internal/auth"
	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/security"
)

func (a *App) handleSavePlex(w http.ResponseWriter, r *http.Request) {
	cfg, _ := a.store.GetConfig()
	cfg.Plex.DisplayName = firstNonEmpty(security.CleanText(r.FormValue("display_name"), 64, true), "Plex")
	if u := r.FormValue("base_url"); u != "" {
		normalized, err := security.ValidatePlexBaseURL(u)
		if err != nil {
			a.setFlash(w, "err", "Plex URL invalid: "+err.Error())
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}
		if normalized != cfg.Plex.BaseURL {
			a.audit(r, "plex_url_changed", "")
		}
		cfg.Plex.BaseURL = normalized
	}
	cfg.Plex.VerifyTLS = r.FormValue("verify_tls") != ""
	cfg.Plex.SessionIntegration = r.FormValue("session_integration") != ""
	cfg.Plex.IncludeStreamCount = r.FormValue("include_stream_count") != ""
	if to, err := strconv.Atoi(r.FormValue("timeout")); err == nil && to >= 1 {
		cfg.Plex.TimeoutSeconds = to
	}
	if mid := r.FormValue("machine_id"); mid != "" {
		if v, err := security.ValidateMachineIdentifier(mid); err == nil {
			cfg.Plex.ExpectedMachineID = v
		}
	} else if r.FormValue("clear_machine_id") != "" {
		cfg.Plex.ExpectedMachineID = ""
	}

	// Token: replace or remove.
	if r.FormValue("remove_token") != "" {
		cfg.Plex.PlexToken = ""
		cfg.Plex.CachedServerName = ""
		a.audit(r, "plex_token_removed", "")
	} else if tok := r.FormValue("token"); tok != "" {
		hadToken := cfg.Plex.HasToken()
		cfg.Plex.PlexToken = tok
		if hadToken {
			a.audit(r, "plex_token_replaced", "")
		} else {
			a.audit(r, "plex_token_added", "")
		}
	}
	if err := cfg.Validate(); err != nil {
		a.setFlash(w, "err", err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	_ = a.store.SaveConfig(cfg)
	a.setFlash(w, "ok", "Plex settings saved.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (a *App) handleSaveDiscord(w http.ResponseWriter, r *http.Request) {
	cfg, _ := a.store.GetConfig()
	if r.FormValue("remove_webhook") != "" {
		cfg.Discord.WebhookURL = ""
		a.audit(r, "discord_webhook_removed", "")
	} else if wh := r.FormValue("webhook"); wh != "" {
		normalized, err := security.ValidateDiscordWebhookURL(wh)
		if err != nil {
			a.setFlash(w, "err", "Webhook invalid: "+err.Error())
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}
		had := cfg.Discord.HasWebhook()
		cfg.Discord.WebhookURL = normalized
		if had {
			a.audit(r, "discord_webhook_replaced", "")
		} else {
			a.audit(r, "discord_webhook_added", "")
		}
	}
	cfg.Discord.UsernameOverride = security.CleanText(r.FormValue("username"), 80, true)
	cfg.Discord.AvatarURL = security.CleanText(r.FormValue("avatar_url"), 500, true)
	cfg.Discord.DefaultMention = validMention(r.FormValue("default_mention"))
	if v, err := security.ValidateRoleID(r.FormValue("role_id")); err == nil {
		cfg.Discord.RoleID = v
	} else if r.FormValue("role_id") == "" {
		cfg.Discord.RoleID = ""
	}
	cfg.Monitoring.AutoOutageNotify = r.FormValue("auto_outage") != ""
	cfg.Monitoring.AutoRecoveryNotify = r.FormValue("auto_recovery") != ""
	cfg.Monitoring.AutoOutageMention = validMention(r.FormValue("auto_outage_mention"))
	cfg.Monitoring.AutoRecoveryMention = validMention(r.FormValue("auto_recovery_mention"))
	_ = a.store.SaveConfig(cfg)
	a.setFlash(w, "ok", "Discord settings saved.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (a *App) handleSaveMonitoring(w http.ResponseWriter, r *http.Request) {
	cfg, _ := a.store.GetConfig()
	preset := r.FormValue("preset")
	cfg.Monitoring = config.ApplyPreset(cfg.Monitoring, preset)
	if preset == "custom" {
		applyCustomMonitoring(&cfg, r)
	}
	if v, err := strconv.Atoi(r.FormValue("stabilization")); err == nil {
		cfg.Monitoring.StabilizationSeconds = v
	}
	if v, err := strconv.Atoi(r.FormValue("cooldown")); err == nil {
		cfg.Monitoring.CooldownSeconds = v
	}
	if v, err := strconv.Atoi(r.FormValue("latency_warn")); err == nil {
		cfg.Monitoring.LatencyWarnMillis = v
	}
	cfg.Monitoring.SupplementalHostDiag = r.FormValue("supplemental_diag") != ""
	cfg.Monitoring.DegradedEnabled = r.FormValue("degraded_enabled") != ""
	if err := cfg.Monitoring.Validate(); err != nil {
		a.setFlash(w, "err", "Monitoring settings invalid: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	_ = a.store.SaveConfig(cfg)
	a.audit(r, "monitoring_thresholds_changed", cfg.Monitoring.Preset)
	a.setFlash(w, "ok", "Monitoring settings saved.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (a *App) handleSaveGeneral(w http.ResponseWriter, r *http.Request) {
	cfg, _ := a.store.GetConfig()
	cfg.General.SiteTitle = firstNonEmpty(security.CleanText(r.FormValue("site_title"), 60, true), "ReelPing")
	if tz := r.FormValue("time_zone"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			cfg.General.TimeZone = tz
		}
	}
	if df := r.FormValue("date_format"); df != "" {
		cfg.General.DateFormat = security.CleanText(df, 60, true)
	}
	switch config.Theme(r.FormValue("theme")) {
	case config.ThemeLight:
		cfg.General.Theme = config.ThemeLight
	case config.ThemeDark:
		cfg.General.Theme = config.ThemeDark
	default:
		cfg.General.Theme = config.ThemeSystem
	}
	if v, err := strconv.Atoi(r.FormValue("retention_announcements")); err == nil {
		cfg.General.RetentionDays.AnnouncementsDays = v
	}
	if v, err := strconv.Atoi(r.FormValue("retention_audit")); err == nil {
		cfg.General.RetentionDays.AuditDays = v
	}
	if v, err := strconv.Atoi(r.FormValue("retention_notifications")); err == nil {
		cfg.General.RetentionDays.NotificationsDays = v
	}
	// Trusted proxies (comma/space separated).
	cfg.Security.TrustedProxies = splitList(r.FormValue("trusted_proxies"))
	a.trustedProxies = cfg.Security.TrustedProxies
	if err := cfg.Validate(); err != nil {
		a.setFlash(w, "err", err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	_ = a.store.SaveConfig(cfg)
	a.setFlash(w, "ok", "General settings saved.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	admin, err := a.store.GetAdmin()
	if err != nil {
		a.setFlash(w, "err", "Could not load account.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if err := auth.VerifyPassword(r.FormValue("current_password"), admin.PasswordHash); err != nil {
		a.audit(r, "password_change", "failed: wrong current password")
		a.setFlash(w, "err", "Current password is incorrect.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	newPw := r.FormValue("new_password")
	if newPw != r.FormValue("confirm_password") {
		a.setFlash(w, "err", "New passwords do not match.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if ps := auth.CheckPassword(newPw); !ps.Acceptable {
		a.setFlash(w, "err", "New password not strong enough: "+ps.Reason)
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPassword(newPw)
	if err != nil {
		a.setFlash(w, "err", "Could not update password.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	admin.PasswordHash = hash
	admin.UpdatedAt = time.Now().UTC()
	_ = a.store.SaveAdmin(admin)
	// Invalidate other sessions after a password change.
	_, _ = a.sessions.DeleteOthers(r)
	a.audit(r, "password_change", "success")
	a.setFlash(w, "ok", "Password changed. Other sessions were signed out.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (a *App) handleSignOutOthers(w http.ResponseWriter, r *http.Request) {
	n, _ := a.sessions.DeleteOthers(r)
	a.audit(r, "session_invalidation", strconv.Itoa(n)+" other sessions")
	a.setFlash(w, "ok", "Signed out "+strconv.Itoa(n)+" other session(s).")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (a *App) handleClearHistory(w http.ResponseWriter, r *http.Request) {
	which := r.FormValue("which")
	if err := a.store.ClearHistory(which); err != nil {
		a.setFlash(w, "err", "Could not clear history.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	a.audit(r, "history_cleared", which)
	a.setFlash(w, "ok", "Cleared "+which+" history.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func validMention(s string) config.MentionPolicy {
	mp := config.MentionPolicy(s)
	if mp.Valid() {
		return mp
	}
	return config.MentionNone
}

func splitList(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
