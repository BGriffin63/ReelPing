package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/model"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "reelping.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, path
}

func TestFreshDatabaseSchema(t *testing.T) {
	st, _ := open(t)
	v, err := st.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v != CurrentSchemaVersion {
		t.Fatalf("expected schema %d, got %d", CurrentSchemaVersion, v)
	}
	if id, _ := st.InstallID(); id == "" {
		t.Fatal("expected an install ID")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	st, _ := open(t)
	cfg := config.Default()
	cfg.SetupComplete = true
	cfg.Plex.BaseURL = "http://10.0.0.5:32400"
	cfg.Plex.PlexToken = "tok"
	if err := st.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !got.SetupComplete || got.Plex.BaseURL != cfg.Plex.BaseURL || got.Plex.PlexToken != "tok" {
		t.Fatalf("config did not round-trip: %+v", got.Plex)
	}
}

func TestAdminPersistsHash(t *testing.T) {
	st, _ := open(t)
	a := model.Admin{Username: "admin", PasswordHash: "$argon2id$hash", CreatedAt: time.Now().UTC()}
	if err := st.SaveAdmin(a); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if got.PasswordHash != "$argon2id$hash" {
		t.Fatalf("hash not persisted, got %q", got.PasswordHash)
	}
}

func TestStateAndIncidentSurviveReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reelping.db")

	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	inc := model.Incident{ID: model.NewID(), Service: "Plex", Open: true, ConfirmedOfflineAt: time.Now().UTC()}
	_ = st.PutIncident(inc)
	_ = st.SaveMonitorState(model.MonitorState{State: "offline", ActiveIncidentID: inc.ID, ConsecutiveFailures: 5})
	_ = st.Close()

	// Reopen (simulates a restart).
	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()

	state, ok, _ := st2.GetMonitorState()
	if !ok || state.State != "offline" || state.ActiveIncidentID != inc.ID {
		t.Fatalf("state did not survive reopen: %+v", state)
	}
	got, err := st2.GetIncident(inc.ID)
	if err != nil || !got.Open {
		t.Fatalf("incident did not survive reopen: %+v err=%v", got, err)
	}
	incs, _ := st2.ListIncidents(0, nil)
	if len(incs) != 1 {
		t.Fatalf("expected exactly 1 incident after reopen, got %d", len(incs))
	}
}

func TestBackupCreatesFile(t *testing.T) {
	st, _ := open(t)
	_ = st.SaveConfig(config.Default())
	backup, err := st.Backup()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(backup)
	if err != nil || info.Size() == 0 {
		t.Fatalf("backup not created: %v", err)
	}
}

func TestRetentionPrunesOldRecords(t *testing.T) {
	st, _ := open(t)
	now := time.Now().UTC()
	// Old + new announcements.
	_ = st.PutAnnouncement(model.Announcement{ID: model.NewID(), Time: now.AddDate(0, 0, -400), Title: "old"})
	_ = st.PutAnnouncement(model.Announcement{ID: model.NewID(), Time: now, Title: "new"})

	rep, err := st.ApplyRetention(config.Retention{AnnouncementsDays: 365}, now)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Announcements != 1 {
		t.Fatalf("expected 1 pruned announcement, got %d", rep.Announcements)
	}
	remaining, _ := st.ListAnnouncements(0, nil)
	if len(remaining) != 1 || remaining[0].Title != "new" {
		t.Fatalf("wrong announcement retained: %+v", remaining)
	}
}

func TestIdempotencyDedup(t *testing.T) {
	st, _ := open(t)
	now := time.Now().UTC()
	fresh, _ := st.ReserveIdempotency("key1", now, time.Hour)
	if !fresh {
		t.Fatal("first reservation should be fresh")
	}
	again, _ := st.ReserveIdempotency("key1", now, time.Hour)
	if again {
		t.Fatal("second reservation of same key should not be fresh")
	}
}

func TestClearHistory(t *testing.T) {
	st, _ := open(t)
	_ = st.PutAudit(model.AuditEvent{ID: model.NewID(), Time: time.Now().UTC(), Action: "x"})
	if err := st.ClearHistory("audit"); err != nil {
		t.Fatal(err)
	}
	events, _ := st.ListAudit(0, nil)
	if len(events) != 0 {
		t.Fatalf("expected audit cleared, got %d", len(events))
	}
}
