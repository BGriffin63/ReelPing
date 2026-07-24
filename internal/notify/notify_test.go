package notify

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/discord"
	"github.com/BGriffin63/reelping/internal/storage"
)

func discordMessage() discord.Message {
	return discord.Message{Style: discord.StyleInfo, Title: "hi", Description: "there"}
}

func testStore(t *testing.T) *storage.Store {
	t.Helper()
	st, err := storage.Open(filepath.Join(t.TempDir(), "reelping.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestQuietHoursSuppression(t *testing.T) {
	s := New(testStore(t), func(string, ...any) {})
	cfg := config.Default()
	cfg.General.TimeZone = "UTC"
	cfg.QuietHours = config.QuietHoursConfig{
		Enabled: true, Start: "22:00", End: "07:00", Days: []int{0, 1, 2, 3, 4, 5, 6}, AllowCritical: true,
	}

	// 23:00 UTC on a covered day -> inside window.
	night := time.Date(2026, 1, 5, 23, 0, 0, 0, time.UTC)
	if sup, _ := s.quietHoursSuppresses(cfg, night, false); !sup {
		t.Fatal("expected suppression at 23:00 inside quiet hours")
	}
	// Critical bypass allowed.
	if sup, _ := s.quietHoursSuppresses(cfg, night, true); sup {
		t.Fatal("critical should bypass quiet hours when allowed")
	}
	// Midday -> not suppressed.
	noon := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	if sup, _ := s.quietHoursSuppresses(cfg, noon, false); sup {
		t.Fatal("did not expect suppression at noon")
	}
}

func TestDeliverWithoutWebhookRecordsSuppressed(t *testing.T) {
	store := testStore(t)
	// Config with no webhook.
	_ = store.SaveConfig(config.Default())
	s := New(store, func(string, ...any) {})

	sent := s.Deliver(context.Background(), "custom", discordMessage(), "", false)
	if sent {
		t.Fatal("delivery without a webhook must not report success")
	}
	notes, _ := store.ListNotifications(0, nil)
	if len(notes) != 1 || !notes[0].Suppressed {
		t.Fatalf("expected one suppressed notification, got %+v", notes)
	}
}
