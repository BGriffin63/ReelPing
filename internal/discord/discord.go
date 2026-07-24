package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/BGriffin63/reelping/internal/security"
)

// Provider is the notification provider interface. Discord is the only
// implementation in v1, but the interface leaves room for future providers.
type Provider interface {
	Name() string
	Send(ctx context.Context, m Message) Result
}

// Result is the sanitised outcome of a send attempt.
type Result struct {
	Success       bool
	ResultCode    string // e.g. "204", "400", "429", "timeout", "tls", "dns", "error"
	RetryCount    int
	RedactedError string
}

// Config configures the Discord provider.
type Config struct {
	WebhookURL       string
	UsernameOverride string
	AvatarURL        string
	MaxRetries       int
	// PerAttemptTimeout bounds a single HTTP attempt.
	PerAttemptTimeout time.Duration
}

// Discord is the Discord incoming-webhook provider.
type Discord struct {
	cfg    Config
	client *http.Client
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error
}

// New builds a Discord provider after validating the webhook URL.
func New(cfg Config) (*Discord, error) {
	normalized, err := security.ValidateDiscordWebhookURL(cfg.WebhookURL)
	if err != nil {
		return nil, err
	}
	cfg.WebhookURL = normalized
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 4
	}
	if cfg.PerAttemptTimeout <= 0 {
		cfg.PerAttemptTimeout = 10 * time.Second
	}
	return &Discord{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.PerAttemptTimeout,
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: cfg.PerAttemptTimeout,
				DisableKeepAlives:     true,
				ForceAttemptHTTP2:     true,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("refusing to follow webhook redirect")
			},
		},
		now:   time.Now,
		sleep: sleepCtx,
	}, nil
}

// Name implements Provider.
func (d *Discord) Name() string { return "discord" }

// Send delivers a message with bounded retries, backoff+jitter, and 429
// handling. It never returns a raw error containing the webhook URL.
func (d *Discord) Send(ctx context.Context, m Message) Result {
	payload, err := d.buildPayload(m)
	if err != nil {
		return Result{Success: false, ResultCode: "build_error", RedactedError: d.redact(err.Error())}
	}

	var last Result
	for attempt := 0; attempt <= d.cfg.MaxRetries; attempt++ {
		last.RetryCount = attempt
		code, retryAfter, sendErr := d.attempt(ctx, payload)
		switch {
		case sendErr == nil && code >= 200 && code < 300:
			return Result{Success: true, ResultCode: strconv.Itoa(code), RetryCount: attempt}
		case sendErr == nil && code == http.StatusTooManyRequests:
			last = Result{Success: false, ResultCode: "429", RetryCount: attempt}
			wait := retryAfter
			if wait <= 0 || wait > 30*time.Second {
				wait = d.backoff(attempt)
			}
			if err := d.sleep(ctx, wait); err != nil {
				last.ResultCode = "cancelled"
				return last
			}
			continue
		case sendErr == nil && code >= 400 && code < 500:
			// Permanent client error (bad/expired/deleted webhook). Do not retry.
			return Result{Success: false, ResultCode: strconv.Itoa(code), RetryCount: attempt,
				RedactedError: fmt.Sprintf("Discord returned HTTP %d", code)}
		case sendErr == nil && code >= 500:
			last = Result{Success: false, ResultCode: strconv.Itoa(code), RetryCount: attempt,
				RedactedError: fmt.Sprintf("Discord returned HTTP %d", code)}
		default:
			last = Result{Success: false, ResultCode: classifyErr(sendErr), RetryCount: attempt,
				RedactedError: d.redact(sendErr.Error())}
		}
		// Backoff before the next attempt (unless we've exhausted retries).
		if attempt < d.cfg.MaxRetries {
			if err := d.sleep(ctx, d.backoff(attempt)); err != nil {
				last.ResultCode = "cancelled"
				return last
			}
		}
	}
	return last
}

// attempt performs a single POST and returns (statusCode, retryAfter, error).
func (d *Discord) attempt(ctx context.Context, body []byte) (int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ReelPing (+https://github.com/BGriffin63/reelping)")

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	// Drain (bounded) so the connection can be reused / closed cleanly.
	drained, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode == http.StatusTooManyRequests {
		return resp.StatusCode, parseRetryAfter(resp, drained), nil
	}
	return resp.StatusCode, 0, nil
}

func (d *Discord) backoff(attempt int) time.Duration {
	base := 500 * time.Millisecond
	d2 := base << attempt
	if d2 > 8*time.Second {
		d2 = 8 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(base)))
	return d2 + jitter
}

func (d *Discord) redact(s string) string {
	return security.Redact(s, d.cfg.WebhookURL)
}

// parseRetryAfter extracts the retry delay from a 429 response, preferring the
// JSON body's retry_after (seconds) and falling back to the header.
func parseRetryAfter(resp *http.Response, body []byte) time.Duration {
	var payload struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.RetryAfter > 0 {
		return time.Duration(payload.RetryAfter * float64(time.Second))
	}
	if h := resp.Header.Get("Retry-After"); h != "" {
		if secs, err := strconv.ParseFloat(h, 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second))
		}
	}
	return 0
}

func classifyErr(err error) string {
	if err == nil {
		return "error"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns"
	}
	return "error"
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
