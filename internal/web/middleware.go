package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"

	"github.com/BGriffin63/reelping/internal/auth"
	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/security"
)

type ctxKey string

const nonceKey ctxKey = "nonce"

func nonceFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(nonceKey).(string); ok {
		return v
	}
	return ""
}

// baseMiddleware applies per-request nonce, security headers, HTTPS awareness
// (only trusting forwarded headers from configured proxies), and panic
// recovery. It also redirects to /setup until an administrator exists.
func (a *App) baseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				a.logger.Printf("panic serving %s: %v", r.URL.Path, rec)
				http.Error(w, "Internal server error.", http.StatusInternalServerError)
			}
		}()

		// Reflect forwarded-proto only from trusted proxies.
		if a.isTrustedProxy(r) {
			if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
				r.Header.Set("X-Forwarded-Proto", "https")
			}
		} else {
			r.Header.Del("X-Forwarded-Proto")
			r.Header.Del("X-Forwarded-For")
		}

		nonce := security.NewNonce()
		ctx := context.WithValue(r.Context(), nonceKey, nonce)
		r = r.WithContext(ctx)

		authenticated := !isPublicPath(r.URL.Path)
		security.SecurityHeaders(w, nonce, authenticated)

		// First-run gate: force setup until an admin exists.
		hasAdmin, _ := a.store.HasAdmin()
		if !hasAdmin && !isSetupOrStatic(r.URL.Path) {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		if hasAdmin && strings.HasPrefix(r.URL.Path, "/setup") {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isPublicPath(p string) bool {
	return p == "/healthz" || strings.HasPrefix(p, "/static/") || p == "/login" || strings.HasPrefix(p, "/setup")
}

func isSetupOrStatic(p string) bool {
	return strings.HasPrefix(p, "/setup") || strings.HasPrefix(p, "/static/") || p == "/healthz"
}

// currentSession returns the validated session for the request.
func (a *App) currentSession(r *http.Request) (model.Session, bool) {
	// Validate is safe to call with a throwaway ResponseWriter for reads.
	return a.sessions.Validate(discardWriter{}, r)
}

// requireAuthFunc wraps a handler requiring a valid session.
func (a *App) requireAuthFunc(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.sessions.Validate(w, r); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

// protected wraps a state-changing handler: requires auth AND a valid CSRF
// token. It parses the form first so the token is available.
func (a *App) protected(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := a.sessions.Validate(w, r)
		if !ok {
			http.Error(w, "Not authenticated.", http.StatusUnauthorized)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request.", http.StatusBadRequest)
			return
		}
		if !auth.CheckCSRF(sess, r.FormValue("csrf_token")) {
			a.logger.Printf("CSRF rejected for %s from %s", r.URL.Path, a.clientIPTag(r))
			http.Error(w, "Invalid or missing CSRF token.", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

// setupOnly permits a handler only before an administrator exists (used for the
// wizard's connection-test endpoints, which run before any session exists).
func (a *App) setupOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if has, _ := a.store.HasAdmin(); has {
			http.Error(w, "Setup already complete.", http.StatusForbidden)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request.", http.StatusBadRequest)
			return
		}
		h(w, r)
	}
}

// isTrustedProxy reports whether the request's direct peer is a configured
// trusted proxy.
func (a *App) isTrustedProxy(r *http.Request) bool {
	if len(a.trustedProxies) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	for _, p := range a.trustedProxies {
		if strings.EqualFold(strings.TrimSpace(p), host) {
			return true
		}
		if _, cidr, err := net.ParseCIDR(strings.TrimSpace(p)); err == nil && ip != nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIPTag returns a privacy-preserving tag of the client IP (a short hash),
// never the raw address, for audit/session records.
func (a *App) clientIPTag(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if a.isTrustedProxy(r) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			host = strings.TrimSpace(parts[0])
		}
	}
	sum := sha256.Sum256([]byte(host))
	return hex.EncodeToString(sum[:4])
}

// discardWriter is a no-op ResponseWriter used for read-only session validation.
type discardWriter struct{}

func (discardWriter) Header() http.Header         { return http.Header{} }
func (discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (discardWriter) WriteHeader(int)             {}
