package security

import "strings"

// RedactHint returns a safe display hint for a secret: a fixed-width mask plus
// the last 4 characters, e.g. "••••abcd". For very short secrets it returns a
// fully-masked value so nothing meaningful leaks. The empty string yields "".
//
// This is the ONLY representation of a stored secret that is ever shown in the
// UI. Full secret values are never rendered, logged, exported, or returned by
// any API.
func RedactHint(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	const mask = "••••"
	if len(secret) < 8 {
		return mask
	}
	return mask + secret[len(secret)-4:]
}

// Redact replaces any occurrences of the provided secrets within s with a fixed
// placeholder. It is used defensively when composing diagnostic/log strings so
// that a secret can never appear even if accidentally interpolated.
func Redact(s string, secrets ...string) string {
	for _, sec := range secrets {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		s = strings.ReplaceAll(s, sec, "[REDACTED]")
	}
	return s
}
