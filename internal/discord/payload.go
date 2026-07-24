package discord

import (
	"encoding/json"
	"time"
)

// webhookPayload is the JSON body posted to a Discord webhook.
type webhookPayload struct {
	Content         string          `json:"content,omitempty"`
	Username        string          `json:"username,omitempty"`
	AvatarURL       string          `json:"avatar_url,omitempty"`
	Embeds          []embed         `json:"embeds,omitempty"`
	AllowedMentions allowedMentions `json:"allowed_mentions"`
}

type embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color,omitempty"`
	Fields      []embedField `json:"fields,omitempty"`
	Footer      *embedFooter `json:"footer,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type embedFooter struct {
	Text string `json:"text"`
}

// allowedMentions is always sent explicitly. A nil/empty Parse suppresses all
// mentions; entries opt specific categories back in.
type allowedMentions struct {
	Parse []string `json:"parse"`
	Roles []string `json:"roles,omitempty"`
	Users []string `json:"users,omitempty"`
}

// buildPayload converts a Message into the JSON body, applying escaping,
// per-field limits, and the 6000-character total budget.
func (d *Discord) buildPayload(m Message) ([]byte, error) {
	content, mentions := buildContent(m.MentionPolicy, m.RoleID)

	title := clamp(m.Style.Symbol()+" "+escapeMarkdown(m.Title), limitTitle)
	desc := clamp(escapeMarkdown(m.Description), limitDescription)

	e := embed{
		Title:       title,
		Description: desc,
		Color:       m.Style.Color(),
	}
	if !m.Timestamp.IsZero() {
		e.Timestamp = m.Timestamp.UTC().Format(time.RFC3339)
	}
	footer := m.FooterText
	if footer == "" {
		footer = "ReelPing • Keep your viewers in the loop"
	}
	e.Footer = &embedFooter{Text: clamp(footer, limitFooter)}

	// Budget tracking against the 6000-char total across the embed.
	used := len(title) + len(desc) + len(e.Footer.Text)
	for _, f := range m.Fields {
		if len(e.Fields) >= limitFields {
			break
		}
		name := clamp(escapeMarkdown(f.Name), limitFieldName)
		value := clamp(escapeMarkdown(f.Value), limitFieldValue)
		if value == "" {
			value = "—"
		}
		cost := len(name) + len(value)
		if used+cost > limitTotalChars {
			break
		}
		used += cost
		e.Fields = append(e.Fields, embedField{Name: name, Value: value, Inline: f.Inline})
	}

	if len(content) > limitContent {
		content = content[:limitContent]
	}

	payload := webhookPayload{
		Content:         content,
		Username:        clamp(d.cfg.UsernameOverride, 80),
		AvatarURL:       d.cfg.AvatarURL,
		Embeds:          []embed{e},
		AllowedMentions: mentions,
	}
	return json.Marshal(payload)
}
