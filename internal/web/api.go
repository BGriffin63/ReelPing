package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/BGriffin63/reelping/internal/discord"
	"github.com/BGriffin63/reelping/internal/plex"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// handleTestPlex tests a Plex connection using posted values (falling back to
// saved config), and returns a JSON result. It never echoes the token.
func (a *App) handleTestPlex(w http.ResponseWriter, r *http.Request) {
	cfg, _ := a.store.GetConfig()
	baseURL := r.FormValue("base_url")
	if baseURL == "" {
		baseURL = cfg.Plex.BaseURL
	}
	token := r.FormValue("token")
	if token == "" {
		token = cfg.Plex.PlexToken
	}
	timeout := cfg.Plex.TimeoutSeconds
	if to, err := strconv.Atoi(r.FormValue("timeout")); err == nil && to >= 1 {
		timeout = to
	}
	opts := plex.Options{
		BaseURL:       baseURL,
		Token:         token,
		VerifyTLS:     r.FormValue("verify_tls") != "" || (r.FormValue("base_url") == "" && cfg.Plex.VerifyTLS),
		Timeout:       time.Duration(timeout) * time.Second,
		FetchSessions: token != "",
	}
	client, err := plex.New(opts)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "message": "Invalid Plex URL: " + err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeout)*time.Second*4+2*time.Second)
	defer cancel()
	res := client.Check(ctx)
	msg := res.Detail
	if res.OK {
		msg = "Connected to Plex."
		if res.ServerName != "" {
			msg += " Server: " + res.ServerName + "."
		}
		if res.ServerVersion != "" {
			msg += " Version " + res.ServerVersion + "."
		}
		if res.MachineID != "" {
			msg += " Machine ID: " + res.MachineID
		}
	}
	writeJSON(w, map[string]any{
		"ok":             res.OK,
		"message":        msg,
		"classification": string(res.Classification),
		"machine_id":     res.MachineID,
		"server_name":    res.ServerName,
	})
}

// handleTestDiscord sends a test webhook message.
func (a *App) handleTestDiscord(w http.ResponseWriter, r *http.Request) {
	cfg, _ := a.store.GetConfig()
	webhook := r.FormValue("webhook")
	if webhook == "" {
		webhook = cfg.Discord.WebhookURL
	}
	prov, err := discord.New(discord.Config{
		WebhookURL:       webhook,
		UsernameOverride: cfg.Discord.UsernameOverride,
		AvatarURL:        cfg.Discord.AvatarURL,
	})
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "message": "Invalid webhook: " + err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	res := prov.Send(ctx, discord.Message{
		Style:       discord.StyleTest,
		Title:       "ReelPing test successful",
		Description: "If you can see this message, ReelPing can reach your Discord channel.",
	})
	a.audit(r, "diagnostics_generated", "discord test")
	if res.Success {
		writeJSON(w, map[string]any{"ok": true, "message": "ReelPing test successful — check your Discord channel."})
		return
	}
	writeJSON(w, map[string]any{"ok": false, "message": "Delivery failed (" + res.ResultCode + ")."})
}
