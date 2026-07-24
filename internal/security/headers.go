package security

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

// NewNonce returns a fresh base64 CSP nonce for per-response inline styles.
func NewNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// SecurityHeaders sets a strict, self-hosted-friendly set of security headers.
// The CSP allows only same-origin resources; inline <style>/<script> must carry
// the provided nonce. authenticated controls whether caching is fully disabled.
func SecurityHeaders(w http.ResponseWriter, nonce string, authenticated bool) {
	h := w.Header()
	csp := "default-src 'self'; " +
		"script-src 'self' 'nonce-" + nonce + "'; " +
		"style-src 'self' 'nonce-" + nonce + "'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"form-action 'self'; " +
		"base-uri 'none'; " +
		"frame-ancestors 'none'; " +
		"object-src 'none'"
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), interest-cohort=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	if authenticated {
		h.Set("Cache-Control", "no-store, max-age=0")
		h.Set("Pragma", "no-cache")
	}
}
