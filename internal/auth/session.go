package auth

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/BGriffin63/reelping/internal/model"
	"github.com/BGriffin63/reelping/internal/storage"
)

// CookieName is the session cookie name.
const CookieName = "reelping_session"

// SessionStore is the persistence surface the session manager needs.
type SessionStore interface {
	PutSession(model.Session) error
	GetSession(id string) (model.Session, error)
	DeleteSession(id string) error
	DeleteSessionsExcept(keepID string) (int, error)
	PurgeExpiredSessions(now time.Time) (int, error)
}

// Manager creates and validates sessions.
type Manager struct {
	store       SessionStore
	idleTimeout time.Duration
	absTimeout  time.Duration
	now         func() time.Time
}

// NewManager builds a session manager. idle and absolute are the sliding idle
// timeout and the absolute lifetime.
func NewManager(store SessionStore, idle, absolute time.Duration) *Manager {
	return &Manager{
		store:       store,
		idleTimeout: idle,
		absTimeout:  absolute,
		now:         time.Now,
	}
}

// SetClock overrides the clock (tests).
func (m *Manager) SetClock(now func() time.Time) { m.now = now }

// Create starts a new session for username, sets the cookie, and returns the
// session. Callers should first delete any prior session for the same browser
// (Rotate) to prevent session fixation.
func (m *Manager) Create(w http.ResponseWriter, r *http.Request, username string) (model.Session, error) {
	now := m.now().UTC()
	sess := model.Session{
		ID:           model.NewSessionID(),
		Username:     username,
		CreatedAt:    now,
		LastSeenAt:   now,
		ExpiresAt:    now.Add(m.absTimeout),
		CSRFToken:    model.NewCSRFToken(),
		UserAgentTag: tagUserAgent(r),
		IPTag:        "", // populated by the web layer with a privacy tag
	}
	if err := m.store.PutSession(sess); err != nil {
		return model.Session{}, err
	}
	m.setCookie(w, r, sess.ID, sess.ExpiresAt)
	return sess, nil
}

// Validate returns the current valid session, or ok=false. It enforces both the
// idle timeout and absolute expiry, and slides LastSeenAt forward.
func (m *Manager) Validate(w http.ResponseWriter, r *http.Request) (model.Session, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return model.Session{}, false
	}
	sess, err := m.store.GetSession(c.Value)
	if err != nil {
		return model.Session{}, false
	}
	now := m.now().UTC()
	if now.After(sess.ExpiresAt) || now.Sub(sess.LastSeenAt) > m.idleTimeout {
		_ = m.store.DeleteSession(sess.ID)
		m.clearCookie(w, r)
		return model.Session{}, false
	}
	// Slide idle window (persist at most once per minute to limit writes).
	if now.Sub(sess.LastSeenAt) > time.Minute {
		sess.LastSeenAt = now
		_ = m.store.PutSession(sess)
	}
	return sess, true
}

// Destroy removes the current session and clears the cookie.
func (m *Manager) Destroy(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		_ = m.store.DeleteSession(c.Value)
	}
	m.clearCookie(w, r)
}

// DeleteOthers removes every session except the current one.
func (m *Manager) DeleteOthers(r *http.Request) (int, error) {
	c, err := r.Cookie(CookieName)
	keep := ""
	if err == nil {
		keep = c.Value
	}
	return m.store.DeleteSessionsExcept(keep)
}

// CheckCSRF constant-time compares a submitted token against the session token.
func CheckCSRF(sess model.Session, submitted string) bool {
	if sess.CSRFToken == "" || submitted == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(sess.CSRFToken), []byte(submitted)) == 1
}

func (m *Manager) setCookie(w http.ResponseWriter, r *http.Request, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Manager) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// The web layer only trusts X-Forwarded-Proto when a trusted proxy is
	// configured; it sets r.TLS-equivalent state before reaching here. As a
	// fallback we treat the standard header conservatively (cookie Secure is
	// safe to set even behind plain HTTP proxies that terminate TLS upstream).
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

func tagUserAgent(r *http.Request) string {
	ua := r.UserAgent()
	if len(ua) > 80 {
		ua = ua[:80]
	}
	return ua
}

// Compile-time assertion that *storage.Store satisfies SessionStore.
var _ SessionStore = (*storage.Store)(nil)
