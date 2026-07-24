package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/discord"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/plex"
)

// Sender delivers a built message and reports whether it was actually sent
// (false if suppressed by quiet hours or delivery failed). The implementation
// records the notification and handles suppression policy.
type Sender interface {
	Deliver(ctx context.Context, category string, m discord.Message, relatedID string, critical bool) bool
}

// WorkerStore is the persistence surface the worker needs beyond the Engine.
type WorkerStore interface {
	Store
	GetConfig() (config.Config, error)
}

// CheckFunc runs a single Plex check.
type CheckFunc func(ctx context.Context, cfg config.Config) plex.CheckResult

// Worker owns the single monitoring goroutine.
type Worker struct {
	store  WorkerStore
	engine *Engine
	sender Sender
	check  CheckFunc
	now    func() time.Time
	logf   func(format string, args ...any)

	mu             sync.RWMutex
	lastResult     plex.CheckResult
	stabilizeUntil time.Time
	running        bool
}

// NewWorker builds a worker. check runs a Plex check for a given config; sender
// delivers notifications; logf logs (secrets must never be passed to it).
func NewWorker(store WorkerStore, sender Sender, check CheckFunc, logf func(string, ...any)) *Worker {
	return &Worker{
		store:  store,
		engine: NewEngine(store),
		sender: sender,
		check:  check,
		now:    time.Now,
		logf:   logf,
	}
}

// SetClock overrides the clock (tests).
func (w *Worker) SetClock(now func() time.Time) { w.now = now }

// LastResult returns the most recent check result.
func (w *Worker) LastResult() plex.CheckResult {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastResult
}

// State returns the current persisted monitor state.
func (w *Worker) State() model.MonitorState { return w.engine.LoadState() }

// Run drives the monitoring loop until ctx is cancelled. It re-reads config
// each tick so interval/threshold changes take effect without a restart.
func (w *Worker) Run(ctx context.Context) {
	cfg, _ := w.store.GetConfig()
	w.mu.Lock()
	w.stabilizeUntil = w.now().Add(time.Duration(cfg.Monitoring.StabilizationSeconds) * time.Second)
	w.running = true
	w.mu.Unlock()

	// On startup, if monitoring is off, record disabled and idle-wait.
	interval := time.Duration(cfg.Monitoring.CheckIntervalSeconds) * time.Second
	timer := time.NewTimer(0) // fire immediately for the first check
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		cfg, _ = w.store.GetConfig()
		if !cfg.General.MonitoringOn || !cfg.SetupComplete {
			w.setDisabled()
		} else {
			w.tick(ctx, cfg)
		}

		interval = time.Duration(cfg.Monitoring.CheckIntervalSeconds) * time.Second
		if interval < 5*time.Second {
			interval = 5 * time.Second
		}
		timer.Reset(interval)
	}
}

func (w *Worker) setDisabled() {
	st := w.engine.LoadState()
	if st.ActiveMaintenanceID != "" {
		return // preserve maintenance state
	}
	st.State = StateDisabled
	st.UpdatedAt = w.now().UTC()
	_ = w.store.SaveMonitorState(st)
}

// tick performs one check + evaluation + effect delivery.
func (w *Worker) tick(ctx context.Context, cfg config.Config) {
	timeout := time.Duration(cfg.Monitoring.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	// Give the multi-stage check room for several sub-steps.
	checkCtx, cancel := context.WithTimeout(ctx, timeout*4+2*time.Second)
	res := w.check(checkCtx, cfg)
	cancel()

	w.mu.Lock()
	w.lastResult = res
	stabilizing := w.now().Before(w.stabilizeUntil)
	w.mu.Unlock()

	effects, err := w.engine.Step(res, cfg, w.now(), stabilizing)
	if err != nil {
		if w.logf != nil {
			w.logf("monitor step error: %v", err)
		}
		return
	}
	for _, ef := range effects {
		w.deliver(ctx, cfg, ef)
	}
}

func (w *Worker) deliver(ctx context.Context, cfg config.Config, ef Effect) {
	switch ef.Kind {
	case EffectOutage:
		inc, err := w.store.GetIncident(ef.IncidentID)
		if err != nil {
			return
		}
		sent := w.sender.Deliver(ctx, "outage", OutageMessage(cfg, inc), inc.ID, ef.Critical)
		inc.OutageNotified = sent
		_ = w.store.PutIncident(inc)
		w.markNotified(sent)
	case EffectRecovery:
		inc, err := w.store.GetIncident(ef.IncidentID)
		if err != nil {
			return
		}
		sent := w.sender.Deliver(ctx, "recovery", RecoveryMessage(cfg, inc), inc.ID, ef.Critical)
		inc.RecoveryNotified = sent
		_ = w.store.PutIncident(inc)
		w.markNotified(sent)
	case EffectMaintenanceRecovery:
		msg := discord.Message{
			Style:         discord.StyleRecovery,
			Title:         serviceName(cfg) + " is back online",
			Description:   "Maintenance is complete and the media server is responding again.",
			MentionPolicy: cfg.Monitoring.AutoRecoveryMention,
			RoleID:        cfg.Discord.RoleID,
			Timestamp:     w.now(),
		}
		sent := w.sender.Deliver(ctx, "maintenance_recovery", msg, "", ef.Critical)
		w.markNotified(sent)
	}
}

func (w *Worker) markNotified(sent bool) {
	if !sent {
		return
	}
	st := w.engine.LoadState()
	st.LastNotificationAt = w.now().UTC()
	_ = w.store.SaveMonitorState(st)
}
