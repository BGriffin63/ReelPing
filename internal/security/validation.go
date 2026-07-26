// Package security holds input validation, output sanitisation, secret
// redaction, and HTTP security-header helpers. Every value that crosses a trust
// boundary (browser input, Plex/Discord URLs, machine identifiers, role IDs)
// passes through this package.
package security

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Known Discord webhook hostnames. Anything else is rejected.
var discordWebhookHosts = map[string]bool{
	"discord.com":        true,
	"discordapp.com":     true,
	"canary.discord.com": true,
	"ptb.discord.com":    true,
}

var (
	// ErrEmpty indicates a required value was blank.
	ErrEmpty = errors.New("value is required")
	// ErrControlChars indicates a forbidden control/newline character.
	ErrControlChars = errors.New("value contains control or newline characters")
)

// hasControlChars reports whether s contains ASCII control characters,
// including newlines, tabs, null bytes, and other C0/C1 controls. These are the
// building blocks of header/CRLF/log-injection attacks, so they are rejected in
// URLs, hostnames, IDs and other structured fields.
func hasControlChars(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\r' || r == 0 || r == '\t' {
			return true
		}
		if r < 0x20 || r == 0x7f {
			return true
		}
		if r >= 0x80 && r <= 0x9f {
			return true
		}
	}
	return false
}

// ValidatePlexBaseURL validates a user-supplied Plex base URL. It permits only
// http/https, rejects embedded credentials, control characters, and malformed
// ports, and returns a normalised URL string (scheme://host[:port], no path
// beyond what the user typed for a reverse proxy).
//
// It deliberately does NOT reject private/RFC1918 addresses: Plex is very
// commonly hosted on a LAN. SSRF risk is bounded elsewhere (no body reflection,
// bounded redirects/size/time). See docs/SECURITY.md.
func ValidatePlexBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrEmpty
	}
	if hasControlChars(raw) {
		return "", ErrControlChars
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("could not parse URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.User != nil {
		return "", errors.New("URL must not contain embedded credentials")
	}
	if u.Hostname() == "" {
		return "", errors.New("URL must contain a host")
	}
	if err := validateHost(u.Hostname()); err != nil {
		return "", err
	}
	if p := u.Port(); p != "" {
		if err := validatePortString(p); err != nil {
			return "", err
		}
	}
	// Reject query fragments containing tokens etc. — the base URL should be clean.
	u.Fragment = ""
	u.RawQuery = ""
	// Normalise: drop trailing slash on path so probe paths join cleanly.
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

// ValidateDiscordWebhookURL validates a Discord incoming-webhook URL. It
// requires HTTPS, a known Discord host, the /api/webhooks/{id}/{token} shape,
// no embedded credentials, and no control characters.
func ValidateDiscordWebhookURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrEmpty
	}
	if hasControlChars(raw) {
		return "", ErrControlChars
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("could not parse webhook URL: %w", err)
	}
	if u.Scheme != "https" {
		return "", errors.New("webhook URL must use https (Discord requires HTTPS)")
	}
	if u.User != nil {
		return "", errors.New("webhook URL must not contain embedded credentials")
	}
	host := strings.ToLower(u.Hostname())
	if !discordWebhookHosts[host] {
		return "", fmt.Errorf("host %q is not a recognised Discord webhook host", host)
	}
	// Expect /api/webhooks/{id}/{token}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "webhooks" {
		return "", errors.New("webhook URL path must look like /api/webhooks/{id}/{token}")
	}
	if !isNumeric(parts[2]) || parts[2] == "" {
		return "", errors.New("webhook ID segment must be numeric")
	}
	if len(parts[3]) < 8 {
		return "", errors.New("webhook token segment looks too short")
	}
	return u.String(), nil
}

// validateHost validates a hostname or IP literal (no port).
func validateHost(host string) error {
	if host == "" {
		return errors.New("empty host")
	}
	if hasControlChars(host) {
		return ErrControlChars
	}
	// IP literal?
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}
	// Hostname: labels of [A-Za-z0-9-], dots between, <=253 total.
	if len(host) > 253 {
		return errors.New("hostname too long")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 {
			return errors.New("invalid hostname label length")
		}
		for _, r := range label {
			if !(r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
				return fmt.Errorf("invalid character %q in hostname", r)
			}
		}
	}
	return nil
}

func validatePortString(p string) error {
	n, err := strconv.Atoi(p)
	if err != nil {
		return errors.New("port is not a number")
	}
	return ValidatePort(n)
}

// ValidatePort ensures a port is within the valid TCP range.
func ValidatePort(n int) error {
	if n < 1 || n > 65535 {
		return fmt.Errorf("port %d out of range (1-65535)", n)
	}
	return nil
}

// ValidateRoleID validates a Discord role ID (a numeric snowflake).
func ValidateRoleID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrEmpty
	}
	if !isNumeric(s) {
		return "", errors.New("role ID must be numeric")
	}
	if len(s) < 15 || len(s) > 21 {
		return "", errors.New("role ID has an implausible length")
	}
	return s, nil
}

// ValidateMachineIdentifier validates a Plex machine identifier: a bounded
// string of hex-ish/alphanumeric characters with no control characters.
func ValidateMachineIdentifier(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrEmpty
	}
	if len(s) > 128 {
		return "", errors.New("machine identifier too long")
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return "", fmt.Errorf("invalid character %q in machine identifier", r)
		}
	}
	return s, nil
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CleanText trims, validates UTF-8, strips control characters (except it maps
// newlines to spaces for single-line fields when singleLine is true), and
// enforces a max rune length. It is used for all free-text admin input
// (titles, reasons, messages) before storage. Output is still HTML-escaped at
// render time; this only guarantees the stored value is well-formed.
func CleanText(s string, maxRunes int, singleLine bool) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r':
			if singleLine {
				b.WriteByte(' ')
			} else {
				b.WriteRune('\n')
			}
		case r == '\t':
			b.WriteByte(' ')
		case r == 0:
			// drop null bytes
		case unicode.IsControl(r):
			// drop other control characters
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if maxRunes > 0 {
		out = truncateRunes(out, maxRunes)
	}
	return out
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	count := 0
	for i := range s {
		if count == max {
			return s[:i]
		}
		count++
	}
	return s
}
