package discord

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BGriffin63/reelping/internal/config"
)

// Discord documented limits (see docs/RESEARCH.md).
const (
	limitContent     = 2000
	limitTitle       = 256
	limitDescription = 4096
	limitFieldName   = 256
	limitFieldValue  = 1024
	limitFooter      = 2048
	limitFields      = 25
	limitTotalChars  = 6000
)

// Field is a single embed field.
type Field struct {
	Name   string
	Value  string
	Inline bool
}

// Message is a provider-neutral notification. The provider translates it into a
// concrete payload (e.g. a Discord webhook execution).
type Message struct {
	Style          Style
	Title          string
	Description    string
	Fields         []Field
	FooterText     string
	Timestamp      time.Time
	MentionPolicy  config.MentionPolicy
	RoleID         string
	IdempotencyKey string
}

// buildContent produces the message content line that carries any mention, plus
// the allowed_mentions object controlling what may actually ping. This is the
// single place mentions are decided; embeds never ping regardless.
func buildContent(policy config.MentionPolicy, roleID string) (string, allowedMentions) {
	switch policy {
	case config.MentionEveryone:
		return "@everyone", allowedMentions{Parse: []string{"everyone"}}
	case config.MentionHere:
		return "@here", allowedMentions{Parse: []string{"everyone"}}
	case config.MentionRole:
		if roleID != "" {
			return "<@&" + roleID + ">", allowedMentions{Parse: []string{}, Roles: []string{roleID}}
		}
		return "", allowedMentions{Parse: []string{}}
	default: // MentionNone
		return "", allowedMentions{Parse: []string{}}
	}
}

// escapeMarkdown neutralises Discord markdown control characters in
// user-controlled text so it cannot alter formatting or inject structure. It
// does not need to defend against mentions (embeds do not ping and content is
// controlled), but it does prevent markdown/backtick breakouts.
func escapeMarkdown(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '\\', '*', '_', '~', '`', '|', '>', '<', '#', '@':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func clamp(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	// Reserve one rune for the ellipsis.
	out := make([]rune, 0, max)
	for _, r := range s {
		if len(out) >= max-1 {
			break
		}
		out = append(out, r)
	}
	return string(out) + "…"
}
