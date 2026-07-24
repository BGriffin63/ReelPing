package web

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/security"
)

// diagnostics builds the secret-free diagnostics document.
func (a *App) diagnostics() map[string]any {
	cfg, _ := a.store.GetConfig()
	schema, _ := a.store.SchemaVersion()
	install, _ := a.store.InstallID()
	st := a.worker.State()
	last := a.worker.LastResult()

	// Recent sanitised notification errors (no secrets are stored on these).
	notes, _ := a.store.ListNotifications(10, nil)

	return map[string]any{
		"generated_at":         time.Now().UTC().Format(time.RFC3339),
		"build":                a.build,
		"install_id":           install,
		"schema_version":       schema,
		"config_writable":      a.store.Writable(),
		"config_path":          a.store.Path(),
		"monitor_state":        st,
		"last_check":           last,
		"plex_token_set":       cfg.Plex.HasToken(),
		"plex_identity_ok":     st.IdentityVerified,
		"discord_set":          cfg.Discord.HasWebhook(),
		"time_zone":            cfg.General.TimeZone,
		"safe_config":          cfg.Safe(security.RedactHint),
		"recent_notifications": notes,
	}
}

func (a *App) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	vd := a.newViewData(w, r, "Diagnostics", "diagnostics")
	vd.Data = a.diagnostics()
	a.render(w, "diagnostics", vd)
}

func (a *App) handleDiagnosticsDownload(w http.ResponseWriter, r *http.Request) {
	a.audit(r, "diagnostics_generated", "download")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="reelping-diagnostics.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(a.diagnostics())
}

// handleBackupDownload streams a consistent copy of the database. This is a
// full backup and DOES contain configuration (including secrets) — it is
// admin-only and clearly labelled; treat the file like a password vault.
func (a *App) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	a.audit(r, "backup_created", "download")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="reelping-backup-%s.db"`, time.Now().UTC().Format("20060102T150405Z")))
	if err := a.store.BackupTo(w); err != nil {
		a.logger.Printf("backup failed: %v", err)
	}
}

// handleIncidentsExport exports incident history as JSON or CSV (no secrets).
func (a *App) handleIncidentsExport(w http.ResponseWriter, r *http.Request) {
	incidents, _ := a.store.ListIncidents(0, nil)
	format := r.URL.Query().Get("format")
	a.audit(r, "diagnostics_generated", "incident export "+format)
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="reelping-incidents.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"id", "service", "first_failure", "confirmed_offline", "recovered", "duration_seconds", "classification", "outage_notified", "recovery_notified"})
		for _, inc := range incidents {
			rec := ""
			if inc.RecoveredAt != nil {
				rec = inc.RecoveredAt.Format(time.RFC3339)
			}
			_ = cw.Write([]string{
				inc.ID, inc.Service, inc.FirstFailureAt.Format(time.RFC3339),
				inc.ConfirmedOfflineAt.Format(time.RFC3339), rec,
				strconv.FormatInt(inc.DurationSeconds, 10), inc.Classification,
				strconv.FormatBool(inc.OutageNotified), strconv.FormatBool(inc.RecoveryNotified),
			})
		}
		cw.Flush()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="reelping-incidents.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"incidents": incidents})
}

var _ = model.NewID
