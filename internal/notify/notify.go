// Package notify delivers built messages to the configured provider, applying
// quiet-hours suppression and idempotency, and records every outcome. It
// implements monitoring.Sender.
package notify

import (
	"context"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/discord"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/storage"
)

// Service delivers and records notifications.
type Service struct {
	store *storage.Store
	now   func() time.Time
	logf  func(string, ...any)
}

// New builds a notification service.
func New(store *storage.Store, logf func(string, ...any)) *Service {
	return &Service{store: store, now: time.Now, logf: logf}
}

// SetClock overrides the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Deliver sends a message, honouring quiet hours (critical may bypass) and
// idempotency, and records the outcome. It returns whether the message was
// actually sent.
func (s *Service) Deliver(ctx context.Context, category string, m discord.Message, relatedID string, critical bool) bool {
	cfg, err := s.store.GetConfig()
	if err != nil {
		return false
	}
	now := s.now().UTC()

	if !cfg.Discord.HasWebhook() {
		s.record(model.Notification{Category: category, RelatedID: relatedID, Success: false,
			Suppressed: true, SuppressReason: "no webhook configured", ResultCode: "no_webhook"}, now)
		return false
	}

	// Idempotency: never send the same event twice (e.g. browser refresh).
	if m.IdempotencyKey != "" {
		fresh, _ := s.store.ReserveIdempotency(m.IdempotencyKey, now, 24*time.Hour)
		if !fresh {
			return false
		}
	}

	// Quiet hours.
	if suppressed, reason := s.quietHoursSuppresses(cfg, now, critical); suppressed {
		s.record(model.Notification{Category: category, RelatedID: relatedID, Success: false,
			Suppressed: true, SuppressReason: reason, ResultCode: "quiet_hours"}, now)
		return false
	}

	prov, err := discord.New(discord.Config{
		WebhookURL:       cfg.Discord.WebhookURL,
		UsernameOverride: cfg.Discord.UsernameOverride,
		AvatarURL:        cfg.Discord.AvatarURL,
	})
	if err != nil {
		s.record(model.Notification{Category: category, RelatedID: relatedID, Success: false,
			ResultCode: "invalid_webhook", RedactedError: "configured webhook is invalid"}, now)
		return false
	}

	res := prov.Send(ctx, m)
	s.record(model.Notification{
		Provider:      prov.Name(),
		Category:      category,
		Success:       res.Success,
		ResultCode:    res.ResultCode,
		RetryCount:    res.RetryCount,
		RedactedError: res.RedactedError,
		RelatedID:     relatedID,
	}, now)
	return res.Success
}

// quietHoursSuppresses reports whether the current time falls inside quiet hours
// and the event is not an allowed critical bypass.
func (s *Service) quietHoursSuppresses(cfg config.Config, nowUTC time.Time, critical bool) (bool, string) {
	q := cfg.QuietHours
	if !q.Enabled {
		return false, ""
	}
	if critical && q.AllowCritical {
		return false, ""
	}
	local := nowUTC.In(cfg.Location())
	if !dayEnabled(q.Days, int(local.Weekday())) {
		return false, ""
	}
	start, ok1 := parseHM(q.Start)
	end, ok2 := parseHM(q.End)
	if !ok1 || !ok2 {
		return false, ""
	}
	mins := local.Hour()*60 + local.Minute()
	inWindow := false
	if start <= end {
		inWindow = mins >= start && mins < end
	} else {
		// Window wraps midnight.
		inWindow = mins >= start || mins < end
	}
	if inWindow {
		return true, "quiet hours"
	}
	return false, ""
}

func (s *Service) record(n model.Notification, now time.Time) {
	n.ID = model.NewID()
	n.Time = now
	if n.Provider == "" {
		n.Provider = "discord"
	}
	if err := s.store.PutNotification(n); err != nil && s.logf != nil {
		s.logf("failed to record notification: %v", err)
	}
}

func dayEnabled(days []int, wd int) bool {
	if len(days) == 0 {
		return true
	}
	for _, d := range days {
		if d == wd {
			return true
		}
	}
	return false
}

func parseHM(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}
