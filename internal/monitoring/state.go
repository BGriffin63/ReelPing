// Package monitoring implements ReelPing's persisted availability state machine
// and the polling worker that drives it. The Engine is deliberately pure and
// clock-injectable so its behaviour (thresholds, one-incident-per-outage,
// recovery, maintenance suppression, restart safety) is unit-tested with a
// virtual clock and fake checks — no real waiting or network.
package monitoring

// State values for the monitoring state machine.
const (
	StateDisabled           = "disabled"
	StateInitializing       = "initializing"
	StateUnknown            = "unknown"
	StateOnline             = "online"
	StateSuspect            = "suspect"
	StateOffline            = "offline"
	StateRecovering         = "recovering"
	StateDegraded           = "degraded"
	StateMaintenanceOnline  = "maintenance-online"
	StateMaintenanceOffline = "maintenance-offline"
)

// EffectKind identifies a side effect the worker must carry out (a Discord
// notification). The Engine never sends anything itself.
type EffectKind string

const (
	EffectOutage              EffectKind = "outage"
	EffectRecovery            EffectKind = "recovery"
	EffectMaintenanceRecovery EffectKind = "maintenance_recovery"
)

// Effect is a notification the Engine has decided should be sent.
type Effect struct {
	Kind       EffectKind
	IncidentID string
	Critical   bool
}
