// Package model holds ReelPing's pure persisted domain types. It has no
// behaviour and depends only on the standard library, so both storage and the
// higher-level packages can share these definitions without import cycles.
package model

import "time"

// Admin is the single administrator account. PasswordHash is persisted to the
// private database but is never included in any export or diagnostics bundle
// (those are built from explicit safe projections, not from this struct).
type Admin struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Session is a server-side authenticated session.
type Session struct {
	ID           string    `json:"id"` // random opaque token (also the cookie value)
	Username     string    `json:"username"`
	CreatedAt    time.Time `json:"created_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserAgentTag string    `json:"user_agent_tag"`
	IPTag        string    `json:"ip_tag"`
	CSRFToken    string    `json:"csrf_token"`
}

// MonitorState is the persisted snapshot of the monitoring state machine.
type MonitorState struct {
	State                 string    `json:"state"`
	ConsecutiveFailures   int       `json:"consecutive_failures"`
	ConsecutiveSuccesses  int       `json:"consecutive_successes"`
	LastCheckAt           time.Time `json:"last_check_at"`
	LastSuccessAt         time.Time `json:"last_success_at"`
	LastFailureAt         time.Time `json:"last_failure_at"`
	FirstFailureAt        time.Time `json:"first_failure_at"`
	LastLatencyMillis     int64     `json:"last_latency_millis"`
	LastClassification    string    `json:"last_classification"`
	LastDetail            string    `json:"last_detail"`
	ActiveIncidentID      string    `json:"active_incident_id"`
	ActiveMaintenanceID   string    `json:"active_maintenance_id"`
	MaintenanceSawOffline bool      `json:"maintenance_saw_offline"`
	LatencyBreachCount    int       `json:"latency_breach_count"`
	LastNotificationAt    time.Time `json:"last_notification_at"`
	StreamCount           int       `json:"stream_count"`
	StreamCountKnown      bool      `json:"stream_count_known"`
	IdentityVerified      bool      `json:"identity_verified"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// Incident is a confirmed outage record.
type Incident struct {
	ID                 string     `json:"id"`
	Service            string     `json:"service"`
	FirstFailureAt     time.Time  `json:"first_failure_at"`
	ConfirmedOfflineAt time.Time  `json:"confirmed_offline_at"`
	RecoveredAt        *time.Time `json:"recovered_at,omitempty"`
	LastSuccessAt      time.Time  `json:"last_success_at"`
	DurationSeconds    int64      `json:"duration_seconds"`
	Classification     string     `json:"classification"`
	FailedChecks       int        `json:"failed_checks"`
	Diagnostic         string     `json:"diagnostic"`
	OutageNotified     bool       `json:"outage_notified"`
	RecoveryNotified   bool       `json:"recovery_notified"`
	Open               bool       `json:"open"`
}

// MaintenanceKind distinguishes how a maintenance window was created.
type MaintenanceKind string

const (
	MaintenanceScheduled MaintenanceKind = "scheduled"
	MaintenanceImmediate MaintenanceKind = "immediate"
	MaintenanceOffline   MaintenanceKind = "offline"
)

// MaintenanceState is the lifecycle state of a maintenance window.
type MaintenanceState string

const (
	MaintScheduled MaintenanceState = "scheduled"
	MaintActive    MaintenanceState = "active"
	MaintEnded     MaintenanceState = "ended"
	MaintCancelled MaintenanceState = "cancelled"
)

// Maintenance is a scheduled or active maintenance window.
type Maintenance struct {
	ID                 string           `json:"id"`
	Kind               MaintenanceKind  `json:"kind"`
	State              MaintenanceState `json:"state"`
	Title              string           `json:"title"`
	Reason             string           `json:"reason"`
	ScheduledStart     *time.Time       `json:"scheduled_start,omitempty"`
	EstimatedEnd       *time.Time       `json:"estimated_end,omitempty"`
	ActualStart        *time.Time       `json:"actual_start,omitempty"`
	ActualEnd          *time.Time       `json:"actual_end,omitempty"`
	MentionPolicy      string           `json:"mention_policy"`
	IncludeStreamCount bool             `json:"include_stream_count"`
	AutoRecovery       bool             `json:"auto_recovery"`
	CreatedBy          string           `json:"created_by"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// Announcement is a record of a message sent (or attempted) to Discord.
type Announcement struct {
	ID             string    `json:"id"`
	Time           time.Time `json:"time"`
	Type           string    `json:"type"`
	Title          string    `json:"title"`
	MentionPolicy  string    `json:"mention_policy"`
	DeliveryResult string    `json:"delivery_result"`
	Admin          string    `json:"admin"`
	RelatedID      string    `json:"related_id,omitempty"`
}

// Notification is a sanitised Discord delivery record.
type Notification struct {
	ID             string    `json:"id"`
	Time           time.Time `json:"time"`
	Provider       string    `json:"provider"`
	Category       string    `json:"category"`
	Success        bool      `json:"success"`
	ResultCode     string    `json:"result_code"`
	RetryCount     int       `json:"retry_count"`
	RedactedError  string    `json:"redacted_error,omitempty"`
	RelatedID      string    `json:"related_id,omitempty"`
	Suppressed     bool      `json:"suppressed,omitempty"`
	SuppressReason string    `json:"suppress_reason,omitempty"`
}

// AuditEvent records an administrative or configuration action.
type AuditEvent struct {
	ID     string    `json:"id"`
	Time   time.Time `json:"time"`
	Action string    `json:"action"`
	Actor  string    `json:"actor"`
	Detail string    `json:"detail"`
	IPTag  string    `json:"ip_tag"`
}
