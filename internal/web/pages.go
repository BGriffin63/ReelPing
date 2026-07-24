package web

import (
	"net/http"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/monitoring"
)

func (a *App) activeIncident() (model.Incident, bool) {
	st := a.worker.State()
	if st.ActiveIncidentID == "" {
		return model.Incident{}, false
	}
	inc, err := a.store.GetIncident(st.ActiveIncidentID)
	if err != nil {
		return model.Incident{}, false
	}
	return inc, true
}

func (a *App) activeMaintenance() (model.Maintenance, bool) {
	st := a.worker.State()
	if st.ActiveMaintenanceID == "" {
		return model.Maintenance{}, false
	}
	m, err := a.store.GetMaintenance(st.ActiveMaintenanceID)
	if err != nil {
		return model.Maintenance{}, false
	}
	return m, true
}

func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Overview", "overview")
	cfg := vd.Cfg
	inc, hasInc := a.activeIncident()
	maint, hasMaint := a.activeMaintenance()

	confirmSecs := cfg.Monitoring.ConfirmSeconds()
	var incDuration string
	if hasInc {
		incDuration = monitoring.FormatDuration(int64(time.Since(inc.ConfirmedOfflineAt).Seconds()))
	}
	vd.Data = map[string]any{
		"ActiveIncident":    inc,
		"HasIncident":       hasInc,
		"IncidentDuration":  incDuration,
		"ActiveMaintenance": maint,
		"HasMaintenance":    hasMaint,
		"ConfirmSeconds":    confirmSecs,
		"Preset":            cfg.Monitoring.Preset,
	}
	a.render(w, "dashboard", vd)
}

func (a *App) handleMaintenance(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Maintenance", "maintenance")
	maint, hasMaint := a.activeMaintenance()
	history, _ := a.store.ListMaintenance(50, nil)
	vd.Data = map[string]any{
		"ActiveMaintenance": maint,
		"HasMaintenance":    hasMaint,
		"History":           history,
		"Mentions":          mentionOptions,
		"StreamCountKnown":  vd.State.StreamCountKnown,
		"StreamCount":       vd.State.StreamCount,
	}
	a.render(w, "maintenance", vd)
}

func (a *App) handleAnnouncements(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Announcements", "announcements")
	history, _ := a.store.ListAnnouncements(100, nil)
	vd.Data = map[string]any{
		"History":  history,
		"Mentions": mentionOptions,
		"Styles":   announcementStyles,
	}
	a.render(w, "announcements", vd)
}

func (a *App) handleIncidents(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Incidents", "incidents")
	incidents, _ := a.store.ListIncidents(200, nil)
	vd.Data = map[string]any{"Incidents": incidents}
	a.render(w, "incidents", vd)
}

func (a *App) handleIncidentDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inc, err := a.store.GetIncident(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	vd := a.newViewData(w, r, "Incident "+model.ShortID(id), "incidents")
	vd.Data = map[string]any{"Incident": inc}
	a.render(w, "incident_detail", vd)
}

func (a *App) handleNotifications(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Notification history", "notifications")
	notes, _ := a.store.ListNotifications(200, nil)
	vd.Data = map[string]any{"Notifications": notes}
	a.render(w, "notifications", vd)
}

func (a *App) handleAudit(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Audit log", "audit")
	events, _ := a.store.ListAudit(300, nil)
	vd.Data = map[string]any{"Events": events}
	a.render(w, "audit", vd)
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Settings", "settings")
	sessions, _ := a.store.ListSessions()
	vd.Data = map[string]any{
		"Presets":     config.Presets(),
		"PresetOrder": config.PresetOrder,
		"TimeZones":   commonTimeZones,
		"Mentions":    mentionOptions,
		"Sessions":    sessions,
		"Themes":      []string{"system", "light", "dark"},
	}
	a.render(w, "settings", vd)
}

var mentionOptions = []struct {
	Value config.MentionPolicy
	Label string
}{
	{config.MentionNone, "No mention"},
	{config.MentionHere, "@here"},
	{config.MentionEveryone, "@everyone"},
	{config.MentionRole, "Specific role"},
}

var announcementStyles = []struct {
	Value string
	Label string
}{
	{"info", "Information"},
	{"scheduled", "Scheduled maintenance"},
	{"maintenance", "Maintenance active"},
	{"warning", "Warning"},
	{"degraded", "Degraded"},
	{"outage", "Outage"},
	{"recovery", "Recovery / restored"},
}
