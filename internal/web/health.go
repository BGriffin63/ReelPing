package web

import (
	"encoding/json"
	"net/http"
)

// handleHealthz reports ReelPing's own health. It deliberately does NOT reflect
// Plex availability — a Plex outage must never mark the ReelPing container
// unhealthy. It only confirms the process is up and the database is writable.
func (a *App) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writable := a.store.Writable()
	status := "ok"
	code := http.StatusOK
	if !writable {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          status,
		"version":         a.build.Version,
		"config_writable": writable,
	})
}
