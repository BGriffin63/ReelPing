package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BGriffin63/reelping/internal/config"
)

// newTestProvider builds a Discord provider pointed at an arbitrary URL,
// bypassing host validation (which is covered separately in the security
// package). It uses a no-op sleep so retry tests run instantly.
func newTestProvider(url string) *Discord {
	d, _ := New(Config{WebhookURL: "https://discord.com/api/webhooks/123456789012345678/abcdefghijklmnop"})
	d.cfg.WebhookURL = url
	d.cfg.MaxRetries = 3
	d.sleep = func(context.Context, time.Duration) error { return nil }
	return d
}

func TestSendSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	d := newTestProvider(srv.URL)
	res := d.Send(context.Background(), Message{Style: StyleTest, Title: "t", Description: "d"})
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSendPermanentClientErrorNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	d := newTestProvider(srv.URL)
	res := d.Send(context.Background(), Message{Style: StyleOutage, Title: "t"})
	if res.Success {
		t.Fatalf("404 should not be success")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("404 should not be retried, got %d calls", got)
	}
}

func TestSendRetriesOn429ThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{"retry_after": 0.01})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	d := newTestProvider(srv.URL)
	res := d.Send(context.Background(), Message{Style: StyleTest, Title: "t"})
	if !res.Success {
		t.Fatalf("expected eventual success, got %+v", res)
	}
	if res.RetryCount < 1 {
		t.Fatalf("expected at least one retry, got %d", res.RetryCount)
	}
}

func TestSendRetriesOn500ThenGivesUp(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	d := newTestProvider(srv.URL)
	res := d.Send(context.Background(), Message{Style: StyleTest, Title: "t"})
	if res.Success {
		t.Fatalf("persistent 500 should fail")
	}
	if got := atomic.LoadInt32(&calls); got != 4 { // 1 + 3 retries
		t.Fatalf("expected 4 attempts, got %d", got)
	}
}

func TestAllowedMentionsNone(t *testing.T) {
	d := newTestProvider("http://x")
	body, _ := d.buildPayload(Message{Style: StyleInfo, Title: "t", MentionPolicy: config.MentionNone})
	var p webhookPayload
	_ = json.Unmarshal(body, &p)
	if len(p.AllowedMentions.Parse) != 0 {
		t.Fatalf("none policy must have empty parse, got %v", p.AllowedMentions.Parse)
	}
	if p.Content != "" {
		t.Fatalf("none policy must not add mention content, got %q", p.Content)
	}
}

func TestAllowedMentionsEveryone(t *testing.T) {
	d := newTestProvider("http://x")
	body, _ := d.buildPayload(Message{Style: StyleOutage, Title: "t", MentionPolicy: config.MentionEveryone})
	var p webhookPayload
	_ = json.Unmarshal(body, &p)
	if len(p.AllowedMentions.Parse) != 1 || p.AllowedMentions.Parse[0] != "everyone" {
		t.Fatalf("everyone policy must parse everyone, got %v", p.AllowedMentions.Parse)
	}
	if p.Content != "@everyone" {
		t.Fatalf("expected @everyone content, got %q", p.Content)
	}
}

func TestAllowedMentionsRole(t *testing.T) {
	d := newTestProvider("http://x")
	body, _ := d.buildPayload(Message{Style: StyleOutage, Title: "t", MentionPolicy: config.MentionRole, RoleID: "42"})
	var p webhookPayload
	_ = json.Unmarshal(body, &p)
	if len(p.AllowedMentions.Parse) != 0 {
		t.Fatalf("role policy must not parse everyone/here, got %v", p.AllowedMentions.Parse)
	}
	if len(p.AllowedMentions.Roles) != 1 || p.AllowedMentions.Roles[0] != "42" {
		t.Fatalf("role policy must whitelist the role, got %v", p.AllowedMentions.Roles)
	}
}

func TestMarkdownAndMentionEscaping(t *testing.T) {
	d := newTestProvider("http://x")
	body, _ := d.buildPayload(Message{
		Style:       StyleInfo,
		Title:       "hi",
		Description: "hello @everyone `code` **bold** <@&123>",
	})
	s := string(body)
	if strings.Contains(s, "@everyone") && !strings.Contains(s, "\\@everyone") {
		t.Fatalf("@everyone in description must be escaped, body=%s", s)
	}
}

func TestRedactionHidesWebhook(t *testing.T) {
	secret := "https://discord.com/api/webhooks/123456789012345678/supersecrettoken"
	d, _ := New(Config{WebhookURL: secret})
	got := d.redact("failed to POST " + secret + " boom")
	if strings.Contains(got, "supersecrettoken") {
		t.Fatalf("redact must remove the webhook, got %q", got)
	}
}

func TestTotalCharBudget(t *testing.T) {
	d := newTestProvider("http://x")
	long := strings.Repeat("A", 5000)
	var fields []Field
	for i := 0; i < 25; i++ {
		fields = append(fields, Field{Name: "n", Value: long})
	}
	body, err := d.buildPayload(Message{Style: StyleInfo, Title: "t", Description: long, Fields: fields})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(body) == 0 {
		t.Fatalf("empty body")
	}
	var p webhookPayload
	_ = json.Unmarshal(body, &p)
	// The embed must stay within Discord's total budget.
	total := len([]rune(p.Embeds[0].Title)) + len([]rune(p.Embeds[0].Description))
	for _, f := range p.Embeds[0].Fields {
		total += len([]rune(f.Name)) + len([]rune(f.Value))
	}
	if total > limitTotalChars {
		t.Fatalf("embed exceeds total budget: %d", total)
	}
}
