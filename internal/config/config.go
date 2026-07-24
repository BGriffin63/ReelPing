// Package config defines ReelPing's typed configuration model, the monitoring
// presets, validation, and the "safe" projection used for exports/diagnostics
// (which never contains secrets).
package config

import (
	"errors"
	"fmt"
	"time"
)

// MentionPolicy controls how a Discord message is allowed to mention people.
type MentionPolicy string

const (
	MentionNone     MentionPolicy = "none"
	MentionHere     MentionPolicy = "here"
	MentionEveryone MentionPolicy = "everyone"
	MentionRole     MentionPolicy = "role"
)

// Valid reports whether the policy is a known value.
func (m MentionPolicy) Valid() bool {
	switch m {
	case MentionNone, MentionHere, MentionEveryone, MentionRole:
		return true
	}
	return false
}

// Theme is the UI colour theme preference.
type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

// Config is the complete persisted configuration. It is stored as JSON in the
// bbolt database. Secret fields (PlexToken, DiscordWebhookURL) are included so
// they persist, but the SafeConfig projection strips them for any export.
type Config struct {
	General    GeneralConfig    `json:"general"`
	Plex       PlexConfig       `json:"plex"`
	Monitoring MonitoringConfig `json:"monitoring"`
	Discord    DiscordConfig    `json:"discord"`
	Security   SecurityConfig   `json:"security"`
	QuietHours QuietHoursConfig `json:"quiet_hours"`

	// SetupComplete becomes true once the first-run wizard finishes.
	SetupComplete bool `json:"setup_complete"`
}

// GeneralConfig holds site-wide settings.
type GeneralConfig struct {
	SiteTitle     string    `json:"site_title"`
	TimeZone      string    `json:"time_zone"`
	DateFormat    string    `json:"date_format"`
	Theme         Theme     `json:"theme"`
	BaseURL       string    `json:"base_url"`
	MonitoringOn  bool      `json:"monitoring_on"`
	RetentionDays Retention `json:"retention"`
}

// Retention holds bounded-history settings (all in days; 0 = keep indefinitely
// where noted).
type Retention struct {
	AnnouncementsDays int `json:"announcements_days"`
	AuditDays         int `json:"audit_days"`
	NotificationsDays int `json:"notifications_days"`
	MonitorErrorsDays int `json:"monitor_errors_days"`
	MaxIncidents      int `json:"max_incidents"`
}

// PlexConfig holds Plex connection settings. PlexToken is a secret.
type PlexConfig struct {
	DisplayName        string `json:"display_name"`
	BaseURL            string `json:"base_url"`
	PlexToken          string `json:"plex_token"` // secret
	ExpectedMachineID  string `json:"expected_machine_id"`
	VerifyTLS          bool   `json:"verify_tls"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	SessionIntegration bool   `json:"session_integration"`
	IncludeStreamCount bool   `json:"include_stream_count"`
	// Cached identity discovered during a connection test (non-secret).
	CachedServerName    string `json:"cached_server_name,omitempty"`
	CachedServerVersion string `json:"cached_server_version,omitempty"`
}

// HasToken reports whether a Plex token is configured.
func (p PlexConfig) HasToken() bool { return p.PlexToken != "" }

// MonitoringConfig holds the monitoring thresholds and behaviour.
type MonitoringConfig struct {
	Preset               string `json:"preset"`
	CheckIntervalSeconds int    `json:"check_interval_seconds"`
	TimeoutSeconds       int    `json:"timeout_seconds"`
	FailureThreshold     int    `json:"failure_threshold"`
	RecoveryThreshold    int    `json:"recovery_threshold"`
	StabilizationSeconds int    `json:"stabilization_seconds"`
	CooldownSeconds      int    `json:"cooldown_seconds"`
	LatencyWarnMillis    int    `json:"latency_warn_millis"`
	SupplementalHostDiag bool   `json:"supplemental_host_diag"`

	DegradedEnabled bool `json:"degraded_enabled"`
	DegradedNotify  bool `json:"degraded_notify"`
	LatencyBreaches int  `json:"latency_breaches"`

	AutoOutageNotify    bool          `json:"auto_outage_notify"`
	AutoRecoveryNotify  bool          `json:"auto_recovery_notify"`
	AutoOutageMention   MentionPolicy `json:"auto_outage_mention"`
	AutoRecoveryMention MentionPolicy `json:"auto_recovery_mention"`
}

// ConfirmSeconds returns the approximate outage-confirmation time.
func (m MonitoringConfig) ConfirmSeconds() int {
	return m.CheckIntervalSeconds * m.FailureThreshold
}

// DiscordConfig holds the Discord webhook settings. DiscordWebhookURL is secret.
type DiscordConfig struct {
	WebhookURL       string        `json:"webhook_url"` // secret
	UsernameOverride string        `json:"username_override"`
	AvatarURL        string        `json:"avatar_url"`
	DefaultMention   MentionPolicy `json:"default_mention"`
	RoleID           string        `json:"role_id"`
}

// HasWebhook reports whether a Discord webhook is configured.
func (d DiscordConfig) HasWebhook() bool { return d.WebhookURL != "" }

// SecurityConfig holds auth/session/proxy settings.
type SecurityConfig struct {
	SessionIdleMinutes   int      `json:"session_idle_minutes"`
	SessionAbsoluteHours int      `json:"session_absolute_hours"`
	LoginMaxAttempts     int      `json:"login_max_attempts"`
	LoginWindowSeconds   int      `json:"login_window_seconds"`
	TrustedProxies       []string `json:"trusted_proxies"`
}

// QuietHoursConfig optionally suppresses non-critical notifications.
type QuietHoursConfig struct {
	Enabled       bool   `json:"enabled"`
	Start         string `json:"start"` // "22:00"
	End           string `json:"end"`   // "07:00"
	Days          []int  `json:"days"`  // 0=Sunday..6=Saturday
	AllowCritical bool   `json:"allow_critical"`
}

// Location resolves the configured time zone, falling back to UTC.
func (c Config) Location() *time.Location {
	if c.General.TimeZone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(c.General.TimeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// Default returns a Config populated with safe defaults (Balanced preset,
// monitoring off, no secrets, setup incomplete).
func Default() Config {
	bal := Presets()["balanced"]
	return Config{
		General: GeneralConfig{
			SiteTitle:    "ReelPing",
			TimeZone:     "UTC",
			DateFormat:   "2006-01-02 15:04:05 MST",
			Theme:        ThemeSystem,
			MonitoringOn: false,
			RetentionDays: Retention{
				AnnouncementsDays: 365,
				AuditDays:         90,
				NotificationsDays: 90,
				MonitorErrorsDays: 14,
				MaxIncidents:      0,
			},
		},
		Plex: PlexConfig{
			DisplayName:    "Plex",
			VerifyTLS:      true,
			TimeoutSeconds: 5,
		},
		Monitoring: MonitoringConfig{
			Preset:               "balanced",
			CheckIntervalSeconds: bal.CheckIntervalSeconds,
			TimeoutSeconds:       bal.TimeoutSeconds,
			FailureThreshold:     bal.FailureThreshold,
			RecoveryThreshold:    bal.RecoveryThreshold,
			StabilizationSeconds: 60,
			CooldownSeconds:      300,
			LatencyWarnMillis:    2000,
			SupplementalHostDiag: false,
			DegradedEnabled:      true,
			DegradedNotify:       false,
			LatencyBreaches:      3,
			AutoOutageNotify:     true,
			AutoRecoveryNotify:   true,
			AutoOutageMention:    MentionNone,
			AutoRecoveryMention:  MentionNone,
		},
		Discord: DiscordConfig{
			DefaultMention: MentionNone,
		},
		Security: SecurityConfig{
			SessionIdleMinutes:   120,
			SessionAbsoluteHours: 168,
			LoginMaxAttempts:     5,
			LoginWindowSeconds:   300,
		},
		QuietHours: QuietHoursConfig{
			AllowCritical: true,
			Days:          []int{0, 1, 2, 3, 4, 5, 6},
		},
	}
}

// Validate checks invariants across the whole config.
func (c Config) Validate() error {
	if err := c.Monitoring.Validate(); err != nil {
		return err
	}
	if c.General.TimeZone != "" {
		if _, err := time.LoadLocation(c.General.TimeZone); err != nil {
			return fmt.Errorf("invalid time zone %q", c.General.TimeZone)
		}
	}
	if !c.Discord.DefaultMention.Valid() {
		return errors.New("invalid default mention policy")
	}
	return nil
}

// Validate enforces the safe monitoring bounds from the product spec.
func (m MonitoringConfig) Validate() error {
	if m.CheckIntervalSeconds < 10 {
		return errors.New("check interval must be at least 10 seconds")
	}
	if m.CheckIntervalSeconds > 3600 {
		return errors.New("check interval must be at most 3600 seconds")
	}
	if m.TimeoutSeconds < 1 {
		return errors.New("request timeout must be at least 1 second")
	}
	if m.TimeoutSeconds >= m.CheckIntervalSeconds {
		return errors.New("request timeout must be shorter than the check interval")
	}
	if m.FailureThreshold < 2 {
		return errors.New("failure threshold must be at least 2")
	}
	if m.FailureThreshold > 20 {
		return errors.New("failure threshold must be at most 20")
	}
	if m.RecoveryThreshold < 1 {
		return errors.New("recovery threshold must be at least 1")
	}
	if m.RecoveryThreshold > 20 {
		return errors.New("recovery threshold must be at most 20")
	}
	if m.StabilizationSeconds < 0 || m.StabilizationSeconds > 3600 {
		return errors.New("stabilization must be between 0 and 3600 seconds")
	}
	if m.CooldownSeconds < 0 || m.CooldownSeconds > 86400 {
		return errors.New("cooldown must be between 0 and 86400 seconds")
	}
	if !m.AutoOutageMention.Valid() || !m.AutoRecoveryMention.Valid() {
		return errors.New("invalid automatic mention policy")
	}
	return nil
}
