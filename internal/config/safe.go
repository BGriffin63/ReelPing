package config

// SafeConfig is a secret-free projection of Config used for diagnostics and
// exports. Secrets are represented only by boolean "configured" flags. It must
// never contain the Plex token, the Discord webhook URL, or any other secret.
type SafeConfig struct {
	SetupComplete bool `json:"setup_complete"`

	General GeneralConfig `json:"general"`

	Plex struct {
		DisplayName         string `json:"display_name"`
		BaseURL             string `json:"base_url"`
		TokenConfigured     bool   `json:"token_configured"`
		ExpectedMachineID   string `json:"expected_machine_id"`
		VerifyTLS           bool   `json:"verify_tls"`
		TimeoutSeconds      int    `json:"timeout_seconds"`
		SessionIntegration  bool   `json:"session_integration"`
		IncludeStreamCount  bool   `json:"include_stream_count"`
		CachedServerName    string `json:"cached_server_name,omitempty"`
		CachedServerVersion string `json:"cached_server_version,omitempty"`
	} `json:"plex"`

	Monitoring MonitoringConfig `json:"monitoring"`

	Discord struct {
		WebhookConfigured bool          `json:"webhook_configured"`
		WebhookHint       string        `json:"webhook_hint"`
		UsernameOverride  string        `json:"username_override"`
		AvatarURL         string        `json:"avatar_url"`
		DefaultMention    MentionPolicy `json:"default_mention"`
		RoleID            string        `json:"role_id"`
		ExtraEnabled      bool          `json:"extra_enabled"`
		ExtraLabel        string        `json:"extra_label"`
		ExtraConfigured   bool          `json:"extra_configured"`
		ExtraHint         string        `json:"extra_hint"`
	} `json:"discord"`

	Security   SecurityConfig   `json:"security"`
	QuietHours QuietHoursConfig `json:"quiet_hours"`
}

// Safe builds the secret-free projection. hintFn produces the redacted hint for
// a secret (inject security.RedactHint to avoid an import cycle).
func (c Config) Safe(hintFn func(string) string) SafeConfig {
	var s SafeConfig
	s.SetupComplete = c.SetupComplete
	s.General = c.General
	s.Monitoring = c.Monitoring
	s.Security = c.Security
	s.QuietHours = c.QuietHours

	s.Plex.DisplayName = c.Plex.DisplayName
	s.Plex.BaseURL = c.Plex.BaseURL
	s.Plex.TokenConfigured = c.Plex.HasToken()
	s.Plex.ExpectedMachineID = c.Plex.ExpectedMachineID
	s.Plex.VerifyTLS = c.Plex.VerifyTLS
	s.Plex.TimeoutSeconds = c.Plex.TimeoutSeconds
	s.Plex.SessionIntegration = c.Plex.SessionIntegration
	s.Plex.IncludeStreamCount = c.Plex.IncludeStreamCount
	s.Plex.CachedServerName = c.Plex.CachedServerName
	s.Plex.CachedServerVersion = c.Plex.CachedServerVersion

	s.Discord.WebhookConfigured = c.Discord.HasWebhook()
	if c.Discord.HasWebhook() && hintFn != nil {
		s.Discord.WebhookHint = hintFn(c.Discord.WebhookURL)
	}
	s.Discord.UsernameOverride = c.Discord.UsernameOverride
	s.Discord.AvatarURL = c.Discord.AvatarURL
	s.Discord.DefaultMention = c.Discord.DefaultMention
	s.Discord.RoleID = c.Discord.RoleID
	s.Discord.ExtraEnabled = c.Discord.ExtraEnabled
	s.Discord.ExtraLabel = c.Discord.ExtraLabel
	s.Discord.ExtraConfigured = c.Discord.ExtraURL != ""
	if c.Discord.ExtraURL != "" && hintFn != nil {
		s.Discord.ExtraHint = hintFn(c.Discord.ExtraURL)
	}
	return s
}
