package monitoring

import (
	"fmt"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/discord"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/plex"
)

// FormatTime renders t in the config's time zone using its date format.
func FormatTime(cfg config.Config, t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	f := cfg.General.DateFormat
	if f == "" {
		f = "2006-01-02 15:04:05 MST"
	}
	return t.In(cfg.Location()).Format(f)
}

// FormatDuration renders a duration as "Xm Ys" / "Xh Ym Zs".
func FormatDuration(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// OutageMessage builds the Discord outage notification for an incident.
func OutageMessage(cfg config.Config, inc model.Incident) discord.Message {
	class := plex.Classification(inc.Classification)
	return discord.Message{
		Style:         discord.StyleOutage,
		Title:         serviceName(cfg) + " appears to be offline",
		Description:   fmt.Sprintf("ReelPing could not reach the media server after %d consecutive checks.", inc.FailedChecks),
		MentionPolicy: cfg.Monitoring.AutoOutageMention,
		RoleID:        cfg.Discord.RoleID,
		Timestamp:     inc.ConfirmedOfflineAt,
		Fields: []discord.Field{
			{Name: "Service", Value: serviceName(cfg), Inline: true},
			{Name: "Status", Value: "Offline", Inline: true},
			{Name: "Failure type", Value: class.PlainLanguage(), Inline: true},
			{Name: "Detected", Value: FormatTime(cfg, inc.ConfirmedOfflineAt), Inline: false},
			{Name: "Failed checks", Value: fmt.Sprintf("%d", inc.FailedChecks), Inline: true},
			{Name: "Incident ID", Value: model.ShortID(inc.ID), Inline: true},
		},
	}
}

// RecoveryMessage builds the Discord recovery notification for an incident.
func RecoveryMessage(cfg config.Config, inc model.Incident) discord.Message {
	restored := time.Now()
	if inc.RecoveredAt != nil {
		restored = *inc.RecoveredAt
	}
	return discord.Message{
		Style:         discord.StyleRecovery,
		Title:         serviceName(cfg) + " is back online",
		Description:   "The media server is responding again.",
		MentionPolicy: cfg.Monitoring.AutoRecoveryMention,
		RoleID:        cfg.Discord.RoleID,
		Timestamp:     restored,
		Fields: []discord.Field{
			{Name: "Service", Value: serviceName(cfg), Inline: true},
			{Name: "Status", Value: "Online", Inline: true},
			{Name: "Outage duration", Value: FormatDuration(inc.DurationSeconds), Inline: true},
			{Name: "Restored", Value: FormatTime(cfg, restored), Inline: false},
			{Name: "Incident ID", Value: model.ShortID(inc.ID), Inline: true},
		},
	}
}
