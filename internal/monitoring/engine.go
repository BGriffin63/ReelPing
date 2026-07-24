package monitoring

import (
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/plex"
)

// Engine applies check results to the persisted state machine. It reads and
// writes state/incidents through the Store interface and returns any Effects
// (notifications) for the caller to deliver.
type Engine struct {
	store Store
}

// Store is the persistence surface the Engine needs.
type Store interface {
	GetMonitorState() (model.MonitorState, bool, error)
	SaveMonitorState(model.MonitorState) error
	GetIncident(id string) (model.Incident, error)
	PutIncident(model.Incident) error
	GetMaintenance(id string) (model.Maintenance, error)
}

// NewEngine builds an Engine.
func NewEngine(store Store) *Engine { return &Engine{store: store} }

// LoadState returns the current persisted state, defaulting to a fresh one.
func (e *Engine) LoadState() model.MonitorState {
	st, ok, err := e.store.GetMonitorState()
	if err != nil || !ok {
		return model.MonitorState{State: StateInitializing}
	}
	return st
}

// Step applies one check result and returns any notification effects. When
// stabilizing is true (startup window), no new incident is opened and no
// notification is emitted, but state and counters still update — this is what
// makes a ReelPing restart safe (no false outage, no duplicate incident).
func (e *Engine) Step(res plex.CheckResult, cfg config.Config, now time.Time, stabilizing bool) ([]Effect, error) {
	now = now.UTC()
	st := e.LoadState()

	st.LastCheckAt = now
	st.LastClassification = string(res.Classification)
	st.LastDetail = res.Detail
	st.LastLatencyMillis = res.LatencyMillis
	st.IdentityVerified = res.IdentityVerified
	if res.StreamCountKnown {
		st.StreamCount = res.StreamCount
		st.StreamCountKnown = true
	}
	st.UpdatedAt = now

	// Maintenance mode: unexpected outages are suppressed.
	if st.ActiveMaintenanceID != "" {
		return e.stepMaintenance(&st, res, now)
	}

	var effects []Effect
	var err error
	if res.OK {
		effects, err = e.stepSuccess(&st, cfg, now, stabilizing)
	} else {
		effects, err = e.stepFailure(&st, cfg, now, stabilizing)
	}
	if err != nil {
		return nil, err
	}
	if err := e.store.SaveMonitorState(st); err != nil {
		return nil, err
	}
	return effects, nil
}

func (e *Engine) stepSuccess(st *model.MonitorState, cfg config.Config, now time.Time, stabilizing bool) ([]Effect, error) {
	st.LastSuccessAt = now
	st.ConsecutiveFailures = 0

	// Degraded evaluation (latency) only when otherwise online and enabled.
	degraded := false
	if cfg.Monitoring.DegradedEnabled && cfg.Monitoring.LatencyWarnMillis > 0 &&
		st.LastLatencyMillis > int64(cfg.Monitoring.LatencyWarnMillis) {
		st.LatencyBreachCount++
		if st.LatencyBreachCount >= cfg.Monitoring.LatencyBreaches {
			degraded = true
		}
	} else {
		st.LatencyBreachCount = 0
	}

	if st.ActiveIncidentID == "" {
		if degraded {
			st.State = StateDegraded
		} else {
			st.State = StateOnline
		}
		st.ConsecutiveSuccesses = 0
		st.FirstFailureAt = time.Time{}
		return nil, nil
	}

	// There is an open incident: we are recovering.
	st.ConsecutiveSuccesses++
	st.State = StateRecovering
	if stabilizing || st.ConsecutiveSuccesses < cfg.Monitoring.RecoveryThreshold {
		return nil, nil
	}

	// Confirm recovery: close the incident.
	inc, err := e.store.GetIncident(st.ActiveIncidentID)
	if err != nil {
		// Incident record missing; recover state without an effect.
		st.State = StateOnline
		st.ActiveIncidentID = ""
		st.ConsecutiveSuccesses = 0
		st.FirstFailureAt = time.Time{}
		return nil, nil
	}
	rec := now
	inc.RecoveredAt = &rec
	inc.Open = false
	inc.LastSuccessAt = now
	inc.DurationSeconds = int64(now.Sub(inc.ConfirmedOfflineAt).Seconds())
	if inc.DurationSeconds < 0 {
		inc.DurationSeconds = 0
	}
	if err := e.store.PutIncident(inc); err != nil {
		return nil, err
	}

	st.State = StateOnline
	st.ActiveIncidentID = ""
	st.ConsecutiveSuccesses = 0
	st.FirstFailureAt = time.Time{}

	var effects []Effect
	// Only send recovery if the outage was announced (unless explicitly allowed).
	if cfg.Monitoring.AutoRecoveryNotify && inc.OutageNotified {
		effects = append(effects, Effect{Kind: EffectRecovery, IncidentID: inc.ID, Critical: true})
	}
	return effects, nil
}

func (e *Engine) stepFailure(st *model.MonitorState, cfg config.Config, now time.Time, stabilizing bool) ([]Effect, error) {
	st.LastFailureAt = now
	st.ConsecutiveSuccesses = 0
	st.LatencyBreachCount = 0

	if st.ActiveIncidentID != "" {
		// Already in a confirmed outage (possibly bounced back from recovering).
		st.ConsecutiveFailures++
		st.State = StateOffline
		if inc, err := e.store.GetIncident(st.ActiveIncidentID); err == nil {
			inc.FailedChecks++
			inc.Classification = string(res_classification(st))
			inc.Diagnostic = st.LastDetail
			inc.Open = true
			inc.RecoveredAt = nil
			_ = e.store.PutIncident(inc)
		}
		return nil, nil
	}

	st.ConsecutiveFailures++
	if st.FirstFailureAt.IsZero() {
		st.FirstFailureAt = now
	}

	if st.ConsecutiveFailures < cfg.Monitoring.FailureThreshold || stabilizing {
		st.State = StateSuspect
		return nil, nil
	}

	// Confirm outage: open exactly one incident.
	inc := model.Incident{
		ID:                 model.NewID(),
		Service:            serviceName(cfg),
		FirstFailureAt:     st.FirstFailureAt,
		ConfirmedOfflineAt: now,
		LastSuccessAt:      st.LastSuccessAt,
		Classification:     st.LastClassification,
		FailedChecks:       st.ConsecutiveFailures,
		Diagnostic:         st.LastDetail,
		Open:               true,
	}
	if err := e.store.PutIncident(inc); err != nil {
		return nil, err
	}
	st.ActiveIncidentID = inc.ID
	st.State = StateOffline

	var effects []Effect
	if cfg.Monitoring.AutoOutageNotify {
		effects = append(effects, Effect{Kind: EffectOutage, IncidentID: inc.ID, Critical: true})
	}
	return effects, nil
}

func (e *Engine) stepMaintenance(st *model.MonitorState, res plex.CheckResult, now time.Time) ([]Effect, error) {
	var effects []Effect
	if res.OK {
		wasOffline := st.State == StateMaintenanceOffline
		st.State = StateMaintenanceOnline
		st.LastSuccessAt = now
		st.ConsecutiveFailures = 0
		if wasOffline {
			if m, err := e.store.GetMaintenance(st.ActiveMaintenanceID); err == nil && m.AutoRecovery {
				effects = append(effects, Effect{Kind: EffectMaintenanceRecovery, IncidentID: "", Critical: false})
			}
		}
	} else {
		st.State = StateMaintenanceOffline
		st.LastFailureAt = now
		st.ConsecutiveSuccesses = 0
	}
	if err := e.store.SaveMonitorState(*st); err != nil {
		return nil, err
	}
	return effects, nil
}

// res_classification returns the current classification as a plex.Classification.
func res_classification(st *model.MonitorState) plex.Classification {
	return plex.Classification(st.LastClassification)
}

func serviceName(cfg config.Config) string {
	if cfg.Plex.DisplayName != "" {
		return cfg.Plex.DisplayName
	}
	return "Plex"
}
