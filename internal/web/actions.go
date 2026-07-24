package web

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/discord"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/monitoring"
	"github.com/BGriffin63/reelping/internal/security"
)

// deliverCtx bounds a manual delivery.
func deliverCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func (a *App) handleToggleMonitoring(w http.ResponseWriter, r *http.Request) {
	cfg, _ := a.store.GetConfig()
	enable := r.FormValue("enable") == "1"
	cfg.General.MonitoringOn = enable
	if err := a.store.SaveConfig(cfg); err != nil {
		a.setFlash(w, "err", "Could not update monitoring.")
	} else if enable {
		a.audit(r, "monitoring_enabled", "")
		a.setFlash(w, "ok", "Monitoring enabled.")
	} else {
		a.audit(r, "monitoring_disabled", "")
		a.setFlash(w, "warn", "Monitoring disabled.")
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// mentionFrom resolves the mention policy + role ID from a form.
func (a *App) mentionFrom(r *http.Request, cfg config.Config) (config.MentionPolicy, string) {
	mp := config.MentionPolicy(r.FormValue("mention"))
	if !mp.Valid() {
		mp = config.MentionNone
	}
	roleID := cfg.Discord.RoleID
	if v, err := security.ValidateRoleID(r.FormValue("role_id")); err == nil {
		roleID = v
	}
	return mp, roleID
}

func (a *App) reserveIdem(r *http.Request, action string) bool {
	idem := r.FormValue("idem")
	if idem == "" {
		return true // no idem provided; allow (CSRF still protects)
	}
	fresh, _ := a.store.ReserveIdempotency(idem+":"+action, time.Now().UTC(), 24*time.Hour)
	return fresh
}

func (a *App) handleMaintenanceStart(w http.ResponseWriter, r *http.Request) {
	if !a.reserveIdem(r, "maint_start") {
		a.setFlash(w, "warn", "That action was already submitted.")
		http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
		return
	}
	cfg, _ := a.store.GetConfig()
	mp, roleID := a.mentionFrom(r, cfg)
	reason := security.CleanText(r.FormValue("reason"), 500, false)
	title := firstNonEmpty(security.CleanText(r.FormValue("title"), 200, true), "Scheduled Plex maintenance")
	includeStreams := r.FormValue("include_streams") != "" && cfg.Plex.IncludeStreamCount

	now := time.Now().UTC()
	m := model.Maintenance{
		ID: model.NewID(), Kind: model.MaintenanceImmediate, State: model.MaintActive,
		Title: title, Reason: reason, ActualStart: &now, MentionPolicy: string(mp),
		IncludeStreamCount: includeStreams, AutoRecovery: r.FormValue("auto_recovery") != "",
		CreatedBy: a.username(r), CreatedAt: now, UpdatedAt: now,
	}
	if dur := parseDurationMinutes(r.FormValue("duration_min")); dur > 0 {
		end := now.Add(dur)
		m.EstimatedEnd = &end
	}
	_ = a.store.PutMaintenance(m)
	a.enterMaintenance(m.ID)

	msg := a.maintenanceMessage(cfg, m, discord.StyleMaintActive, "Plex is entering maintenance", "Plex will be unavailable while maintenance is completed.", mp, roleID)
	ctx, cancel := deliverCtx()
	defer cancel()
	a.notifier.Deliver(ctx, "maintenance", msg, m.ID, false)
	a.recordAnnouncement(r, "maintenance_start", msg.Title, string(mp), m.ID)
	a.audit(r, "maintenance_started", title)
	a.setFlash(w, "ok", "Maintenance started and announced.")
	http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
}

func (a *App) handleMaintenanceSchedule(w http.ResponseWriter, r *http.Request) {
	if !a.reserveIdem(r, "maint_sched") {
		a.setFlash(w, "warn", "That action was already submitted.")
		http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
		return
	}
	cfg, _ := a.store.GetConfig()
	mp, roleID := a.mentionFrom(r, cfg)
	title := firstNonEmpty(security.CleanText(r.FormValue("title"), 200, true), "Scheduled Plex maintenance")
	reason := security.CleanText(r.FormValue("reason"), 500, false)
	start := parseLocalTime(r.FormValue("start"), cfg)
	end := parseLocalTime(r.FormValue("estimated_end"), cfg)
	now := time.Now().UTC()
	m := model.Maintenance{
		ID: model.NewID(), Kind: model.MaintenanceScheduled, State: model.MaintScheduled,
		Title: title, Reason: reason, ScheduledStart: start, EstimatedEnd: end,
		MentionPolicy: string(mp), AutoRecovery: r.FormValue("auto_recovery") != "",
		CreatedBy: a.username(r), CreatedAt: now, UpdatedAt: now,
	}
	_ = a.store.PutMaintenance(m)

	if r.FormValue("announce_now") != "" {
		msg := a.maintenanceMessage(cfg, m, discord.StyleScheduled, title, "Plex will be unavailable while server maintenance is completed.", mp, roleID)
		ctx, cancel := deliverCtx()
		defer cancel()
		a.notifier.Deliver(ctx, "scheduled_maintenance", msg, m.ID, false)
		a.recordAnnouncement(r, "maintenance_scheduled", msg.Title, string(mp), m.ID)
	}
	a.audit(r, "maintenance_scheduled", title)
	a.setFlash(w, "ok", "Maintenance scheduled.")
	http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
}

func (a *App) handleGoingOffline(w http.ResponseWriter, r *http.Request) {
	if !a.reserveIdem(r, "going_offline") {
		http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
		return
	}
	cfg, _ := a.store.GetConfig()
	mp, roleID := a.mentionFrom(r, cfg)
	reason := security.CleanText(r.FormValue("reason"), 500, false)
	now := time.Now().UTC()
	m := model.Maintenance{
		ID: model.NewID(), Kind: model.MaintenanceOffline, State: model.MaintActive,
		Title: "Plex is going offline", Reason: reason, ActualStart: &now,
		MentionPolicy: string(mp), CreatedBy: a.username(r), CreatedAt: now, UpdatedAt: now,
	}
	if dur := parseDurationMinutes(r.FormValue("duration_min")); dur > 0 {
		end := now.Add(dur)
		m.EstimatedEnd = &end
	}
	_ = a.store.PutMaintenance(m)
	a.enterMaintenance(m.ID)
	msg := a.maintenanceMessage(cfg, m, discord.StyleWarning, "Plex is going offline", "Plex is being taken offline intentionally.", mp, roleID)
	ctx, cancel := deliverCtx()
	defer cancel()
	a.notifier.Deliver(ctx, "going_offline", msg, m.ID, false)
	a.recordAnnouncement(r, "going_offline", msg.Title, string(mp), m.ID)
	a.audit(r, "maintenance_started", "going offline")
	a.setFlash(w, "ok", "Announced. Maintenance mode is active.")
	http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
}

func (a *App) handleMaintenanceDelay(w http.ResponseWriter, r *http.Request) {
	if !a.reserveIdem(r, "maint_delay") {
		http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
		return
	}
	cfg, _ := a.store.GetConfig()
	mp, roleID := a.mentionFrom(r, cfg)
	m, ok := a.activeMaintenance()
	if !ok {
		a.setFlash(w, "err", "There is no active maintenance to delay.")
		http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
		return
	}
	newEnd := parseLocalTime(r.FormValue("estimated_end"), cfg)
	origStr := "—"
	if m.EstimatedEnd != nil {
		origStr = monitoring.FormatTime(cfg, *m.EstimatedEnd)
	}
	m.EstimatedEnd = newEnd
	m.UpdatedAt = time.Now().UTC()
	_ = a.store.PutMaintenance(m)

	newStr := "—"
	if newEnd != nil {
		newStr = monitoring.FormatTime(cfg, *newEnd)
	}
	msg := discord.Message{
		Style: discord.StyleWarning, Title: "Plex maintenance is taking longer than expected",
		Description:   security.CleanText(r.FormValue("reason"), 500, false),
		MentionPolicy: mp, RoleID: roleID, Timestamp: time.Now(),
		Fields: []discord.Field{
			{Name: "Original estimate", Value: origStr, Inline: true},
			{Name: "Updated estimate", Value: newStr, Inline: true},
		},
	}
	ctx, cancel := deliverCtx()
	defer cancel()
	a.notifier.Deliver(ctx, "maintenance_delay", msg, m.ID, false)
	a.recordAnnouncement(r, "maintenance_delay", msg.Title, string(mp), m.ID)
	a.audit(r, "maintenance_extended", newStr)
	a.setFlash(w, "ok", "Delay update sent.")
	http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
}

func (a *App) handleMaintenanceEnd(w http.ResponseWriter, r *http.Request) {
	m, ok := a.activeMaintenance()
	if !ok {
		a.setFlash(w, "err", "There is no active maintenance.")
		http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
		return
	}
	now := time.Now().UTC()
	m.State = model.MaintEnded
	m.ActualEnd = &now
	m.UpdatedAt = now
	_ = a.store.PutMaintenance(m)
	a.exitMaintenance()
	a.audit(r, "maintenance_ended", m.Title)
	a.setFlash(w, "ok", "Maintenance ended. Normal monitoring resumed.")
	http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
}

func (a *App) handleServiceRestored(w http.ResponseWriter, r *http.Request) {
	if !a.reserveIdem(r, "restore") {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cfg, _ := a.store.GetConfig()
	mp, roleID := a.mentionFrom(r, cfg)
	last := a.worker.LastResult()
	override := r.FormValue("override") != ""
	if !last.OK && !override {
		a.setFlash(w, "warn", "ReelPing still cannot reach Plex. Re-check the box to send anyway.")
		http.Redirect(w, r, "/maintenance", http.StatusSeeOther)
		return
	}
	// End any active maintenance too.
	if m, ok := a.activeMaintenance(); ok {
		now := time.Now().UTC()
		m.State = model.MaintEnded
		m.ActualEnd = &now
		_ = a.store.PutMaintenance(m)
		a.exitMaintenance()
	}
	msg := discord.Message{
		Style: discord.StyleRecovery, Title: serviceDisplay(cfg) + " is back online",
		Description: "The media server is available again.", MentionPolicy: mp, RoleID: roleID, Timestamp: time.Now(),
	}
	ctx, cancel := deliverCtx()
	defer cancel()
	a.notifier.Deliver(ctx, "service_restored", msg, "", false)
	a.recordAnnouncement(r, "service_restored", msg.Title, string(mp), "")
	a.audit(r, "announcement_sent", "service restored")
	a.setFlash(w, "ok", "Service-restored announcement sent.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleCustomAnnounce(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.announceLim.Allow("announce"); !ok {
		a.setFlash(w, "err", "Too many announcements too quickly. Please wait a moment.")
		http.Redirect(w, r, "/announcements", http.StatusSeeOther)
		return
	}
	if !a.reserveIdem(r, "announce") {
		a.setFlash(w, "warn", "That announcement was already sent.")
		http.Redirect(w, r, "/announcements", http.StatusSeeOther)
		return
	}
	cfg, _ := a.store.GetConfig()
	mp, roleID := a.mentionFrom(r, cfg)
	title := security.CleanText(r.FormValue("title"), 200, true)
	body := security.CleanText(r.FormValue("message"), 3500, false)
	if title == "" || body == "" {
		a.setFlash(w, "err", "Title and message are required.")
		http.Redirect(w, r, "/announcements", http.StatusSeeOther)
		return
	}
	style := discord.Style(r.FormValue("style"))
	msg := discord.Message{
		Style: style, Title: title, Description: body,
		MentionPolicy: mp, RoleID: roleID, Timestamp: time.Now(),
	}
	ctx, cancel := deliverCtx()
	defer cancel()
	sent := a.notifier.Deliver(ctx, "custom", msg, "", false)
	a.recordAnnouncement(r, "custom", title, string(mp), "")
	a.audit(r, "announcement_sent", "custom: "+title)
	if sent {
		a.setFlash(w, "ok", "Announcement sent.")
	} else {
		a.setFlash(w, "warn", "Announcement recorded but delivery did not succeed (check Notifications).")
	}
	http.Redirect(w, r, "/announcements", http.StatusSeeOther)
}

// --- maintenance state helpers ---

func (a *App) enterMaintenance(id string) {
	st := a.worker.State()
	st.ActiveMaintenanceID = id
	if a.worker.LastResult().OK {
		st.State = monitoring.StateMaintenanceOnline
	} else {
		st.State = monitoring.StateMaintenanceOffline
	}
	st.UpdatedAt = time.Now().UTC()
	_ = a.store.SaveMonitorState(st)
}

func (a *App) exitMaintenance() {
	st := a.worker.State()
	st.ActiveMaintenanceID = ""
	if a.worker.LastResult().OK {
		st.State = monitoring.StateOnline
	} else {
		st.State = monitoring.StateSuspect
	}
	st.UpdatedAt = time.Now().UTC()
	_ = a.store.SaveMonitorState(st)
}

func (a *App) maintenanceMessage(cfg config.Config, m model.Maintenance, style discord.Style, title, desc string, mp config.MentionPolicy, roleID string) discord.Message {
	fields := []discord.Field{}
	if m.ScheduledStart != nil {
		fields = append(fields, discord.Field{Name: "Starts", Value: monitoring.FormatTime(cfg, *m.ScheduledStart), Inline: true})
	} else if m.ActualStart != nil {
		fields = append(fields, discord.Field{Name: "Started", Value: monitoring.FormatTime(cfg, *m.ActualStart), Inline: true})
	}
	if m.EstimatedEnd != nil {
		fields = append(fields, discord.Field{Name: "Expected return", Value: monitoring.FormatTime(cfg, *m.EstimatedEnd), Inline: true})
	}
	if m.Reason != "" {
		fields = append(fields, discord.Field{Name: "Reason", Value: m.Reason, Inline: false})
	}
	if m.IncludeStreamCount {
		st := a.worker.State()
		if st.StreamCountKnown {
			fields = append(fields, discord.Field{Name: "Active streams", Value: strconv.Itoa(st.StreamCount), Inline: true})
		}
	}
	return discord.Message{
		Style: style, Title: title, Description: desc,
		Fields: fields, MentionPolicy: mp, RoleID: roleID, Timestamp: time.Now(),
	}
}

func (a *App) recordAnnouncement(r *http.Request, typ, title, mention, relatedID string) {
	_ = a.store.PutAnnouncement(model.Announcement{
		ID: model.NewID(), Time: time.Now().UTC(), Type: typ, Title: title,
		MentionPolicy: mention, DeliveryResult: "recorded", Admin: a.username(r), RelatedID: relatedID,
	})
}

func (a *App) username(r *http.Request) string {
	sess, _ := a.currentSession(r)
	return sess.Username
}

func serviceDisplay(cfg config.Config) string {
	if cfg.Plex.DisplayName != "" {
		return cfg.Plex.DisplayName
	}
	return "Plex"
}

func parseDurationMinutes(s string) time.Duration {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 100000 {
		return 0
	}
	return time.Duration(n) * time.Minute
}

// parseLocalTime parses a datetime-local value in the configured zone -> UTC.
func parseLocalTime(s string, cfg config.Config) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", s, cfg.Location())
	if err != nil {
		return nil
	}
	u := t.UTC()
	return &u
}
