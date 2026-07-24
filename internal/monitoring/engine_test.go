package monitoring

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/plex"
	"github.com/BGriffin63/reelping/internal/storage"
)

func testStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := storage.Open(filepath.Join(dir, "reelping.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func balancedCfg() config.Config {
	cfg := config.Default()
	cfg.SetupComplete = true
	cfg.General.MonitoringOn = true
	cfg.Monitoring.AutoOutageNotify = true
	cfg.Monitoring.AutoRecoveryNotify = true
	return cfg
}

func okResult() plex.CheckResult {
	return plex.CheckResult{OK: true, Classification: plex.Online, LatencyMillis: 20}
}
func failResult() plex.CheckResult {
	return plex.CheckResult{OK: false, Classification: plex.PlexServiceUnreachable, Detail: "down"}
}

// feed applies n results and returns all emitted effects.
func feed(t *testing.T, e *Engine, cfg config.Config, clock *time.Time, r plex.CheckResult, n int, stabilizing bool) []Effect {
	t.Helper()
	var all []Effect
	for i := 0; i < n; i++ {
		*clock = clock.Add(20 * time.Second)
		efs, err := e.Step(r, cfg, *clock, stabilizing)
		if err != nil {
			t.Fatalf("step: %v", err)
		}
		all = append(all, efs...)
	}
	return all
}

func TestOutageRequiresThreeFailures(t *testing.T) {
	e := NewEngine(testStore(t))
	cfg := balancedCfg()
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Establish online.
	feed(t, e, cfg, &clock, okResult(), 1, false)

	// One failure then success -> no outage.
	if efs := feed(t, e, cfg, &clock, failResult(), 1, false); len(efs) != 0 {
		t.Fatalf("1 failure should not alert, got %v", efs)
	}
	if efs := feed(t, e, cfg, &clock, okResult(), 1, false); len(efs) != 0 {
		t.Fatalf("recovery from suspect should not alert, got %v", efs)
	}
	if got := e.LoadState().State; got != StateOnline {
		t.Fatalf("expected online, got %s", got)
	}

	// Two failures then success -> no outage.
	feed(t, e, cfg, &clock, failResult(), 2, false)
	if got := e.LoadState().State; got != StateSuspect {
		t.Fatalf("expected suspect after 2 failures, got %s", got)
	}
	if efs := feed(t, e, cfg, &clock, okResult(), 1, false); len(efs) != 0 {
		t.Fatalf("recovery before threshold should not alert")
	}
}

func TestConfirmedOutageEmitsExactlyOneAlert(t *testing.T) {
	store := testStore(t)
	e := NewEngine(store)
	cfg := balancedCfg()
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	feed(t, e, cfg, &clock, okResult(), 1, false)

	// Three consecutive failures -> exactly one outage effect.
	efs := feed(t, e, cfg, &clock, failResult(), 3, false)
	outages := countKind(efs, EffectOutage)
	if outages != 1 {
		t.Fatalf("expected exactly 1 outage effect, got %d (%v)", outages, efs)
	}
	if got := e.LoadState().State; got != StateOffline {
		t.Fatalf("expected offline, got %s", got)
	}

	// Ten more failures -> no additional alerts.
	efs = feed(t, e, cfg, &clock, failResult(), 10, false)
	if n := countKind(efs, EffectOutage); n != 0 {
		t.Fatalf("expected no duplicate outage alerts, got %d", n)
	}

	// Exactly one incident should exist and be open.
	incs, _ := store.ListIncidents(0, nil)
	if len(incs) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incs))
	}
	if !incs[0].Open {
		t.Fatalf("incident should be open")
	}
}

func TestRecoveryRequiresTwoSuccesses(t *testing.T) {
	store := testStore(t)
	e := NewEngine(store)
	cfg := balancedCfg()
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	feed(t, e, cfg, &clock, okResult(), 1, false)
	feed(t, e, cfg, &clock, failResult(), 3, false) // outage

	// Mark the incident as notified (worker would do this after sending) so the
	// recovery notification is allowed.
	incs, _ := store.ListIncidents(0, nil)
	incs[0].OutageNotified = true
	_ = store.PutIncident(incs[0])

	// First success -> recovering, no recovery alert.
	if efs := feed(t, e, cfg, &clock, okResult(), 1, false); len(efs) != 0 {
		t.Fatalf("first success should not send recovery, got %v", efs)
	}
	if got := e.LoadState().State; got != StateRecovering {
		t.Fatalf("expected recovering, got %s", got)
	}

	// Second success -> exactly one recovery alert.
	efs := feed(t, e, cfg, &clock, okResult(), 1, false)
	if n := countKind(efs, EffectRecovery); n != 1 {
		t.Fatalf("expected exactly 1 recovery, got %d (%v)", n, efs)
	}
	if got := e.LoadState().State; got != StateOnline {
		t.Fatalf("expected online, got %s", got)
	}

	// Additional successes -> no duplicate recovery.
	if efs := feed(t, e, cfg, &clock, okResult(), 5, false); len(efs) != 0 {
		t.Fatalf("no duplicate recovery expected, got %v", efs)
	}

	// The incident should be closed with a computed duration.
	inc, _ := store.GetIncident(incs[0].ID)
	if inc.Open {
		t.Fatalf("incident should be closed")
	}
	if inc.RecoveredAt == nil || inc.DurationSeconds <= 0 {
		t.Fatalf("expected recovery time and positive duration, got %+v", inc)
	}
}

func TestNoRecoveryAlertWhenOutageNotNotified(t *testing.T) {
	store := testStore(t)
	e := NewEngine(store)
	cfg := balancedCfg()
	cfg.Monitoring.AutoOutageNotify = false // outage not announced
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	feed(t, e, cfg, &clock, okResult(), 1, false)
	feed(t, e, cfg, &clock, failResult(), 3, false)
	efs := feed(t, e, cfg, &clock, okResult(), 2, false)
	if n := countKind(efs, EffectRecovery); n != 0 {
		t.Fatalf("recovery should be suppressed when outage was not notified, got %d", n)
	}
}

func TestRestartDuringSuspectDoesNotFalselyAlert(t *testing.T) {
	store := testStore(t)
	cfg := balancedCfg()
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	e1 := NewEngine(store)
	feed(t, e1, cfg, &clock, okResult(), 1, false)
	feed(t, e1, cfg, &clock, failResult(), 2, false) // suspect, no incident

	// "Restart": new engine, stabilizing window active.
	e2 := NewEngine(store)
	efs := feed(t, e2, cfg, &clock, failResult(), 5, true) // stabilizing
	if n := countKind(efs, EffectOutage); n != 0 {
		t.Fatalf("no outage alert should fire during stabilization, got %d", n)
	}
	incs, _ := store.ListIncidents(0, nil)
	if len(incs) != 0 {
		t.Fatalf("no incident should be created during stabilization, got %d", len(incs))
	}
}

func TestRestartDuringOutageContinuesSameIncident(t *testing.T) {
	store := testStore(t)
	cfg := balancedCfg()
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	e1 := NewEngine(store)
	feed(t, e1, cfg, &clock, okResult(), 1, false)
	feed(t, e1, cfg, &clock, failResult(), 3, false) // one incident open
	incsBefore, _ := store.ListIncidents(0, nil)
	if len(incsBefore) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incsBefore))
	}

	// Restart during confirmed outage; keep failing during stabilization.
	e2 := NewEngine(store)
	efs := feed(t, e2, cfg, &clock, failResult(), 4, true)
	if n := countKind(efs, EffectOutage); n != 0 {
		t.Fatalf("no new outage alert after restart, got %d", n)
	}
	incsAfter, _ := store.ListIncidents(0, nil)
	if len(incsAfter) != 1 {
		t.Fatalf("no duplicate incident after restart, got %d", len(incsAfter))
	}
	if incsAfter[0].ID != incsBefore[0].ID {
		t.Fatalf("incident ID changed after restart")
	}
}

func TestMaintenanceSuppressesOutage(t *testing.T) {
	store := testStore(t)
	e := NewEngine(store)
	cfg := balancedCfg()
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Enter maintenance by writing state directly (as the web layer would).
	st := e.LoadState()
	st.ActiveMaintenanceID = "maint-1"
	st.State = StateMaintenanceOnline
	_ = store.SaveMonitorState(st)

	efs := feed(t, e, cfg, &clock, failResult(), 6, false)
	if len(efs) != 0 {
		t.Fatalf("maintenance must suppress outage alerts, got %v", efs)
	}
	if got := e.LoadState().State; got != StateMaintenanceOffline {
		t.Fatalf("expected maintenance-offline, got %s", got)
	}
	incs, _ := store.ListIncidents(0, nil)
	if len(incs) != 0 {
		t.Fatalf("no incident during maintenance, got %d", len(incs))
	}
}

func countKind(efs []Effect, kind EffectKind) int {
	n := 0
	for _, e := range efs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}
