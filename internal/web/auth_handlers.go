package web

import (
	"net/http"
	"time"

	"github.com/BGriffin63/reelping/internal/auth"
)

func (a *App) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	vd := a.newViewData(w, r, "Sign in", "login")
	a.render(w, "login", vd)
}

func (a *App) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request.", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	ipTag := a.clientIPTag(r)

	// Rate limit by IP and by username.
	if ok, _ := a.loginLimiter.Allow("ip:" + ipTag); !ok {
		a.audit(r, "login_failure", "rate limited")
		a.renderLoginError(w, r, "Too many attempts. Please wait a few minutes and try again.")
		return
	}
	if ok, _ := a.loginLimiter.Allow("user:" + username); !ok {
		a.renderLoginError(w, r, "Too many attempts. Please wait a few minutes and try again.")
		return
	}

	admin, err := a.store.GetAdmin()
	if err != nil || admin.Username != username {
		_ = auth.VerifyPassword(password, dummyHash) // constant-time-ish to reduce user enumeration
		a.audit(r, "login_failure", "unknown user or bad credentials")
		a.renderLoginError(w, r, "Invalid username or password.")
		return
	}
	if err := auth.VerifyPassword(password, admin.PasswordHash); err != nil {
		a.audit(r, "login_failure", "bad password")
		a.renderLoginError(w, r, "Invalid username or password.")
		return
	}

	// Success: rotate session (prevent fixation) and record.
	a.sessions.Destroy(w, r)
	sess, err := a.sessions.Create(w, r, admin.Username)
	if err != nil {
		a.renderLoginError(w, r, "Could not create a session. Please try again.")
		return
	}
	sess.IPTag = ipTag
	_ = a.store.PutSession(sess)
	a.loginLimiter.Reset("ip:" + ipTag)
	a.loginLimiter.Reset("user:" + username)
	a.audit(r, "login_success", "")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.audit(r, "logout", "")
	a.sessions.Destroy(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) renderLoginError(w http.ResponseWriter, r *http.Request, msg string) {
	vd := a.newViewData(w, r, "Sign in", "login")
	vd.Flash = &flash{Kind: "err", Message: msg}
	w.WriteHeader(http.StatusUnauthorized)
	a.render(w, "login", vd)
}

// dummyHash is a valid Argon2id hash of a random value, used to keep timing
// roughly constant when the username is unknown.
var dummyHash = mustHash()

func mustHash() string {
	h, _ := auth.HashPassword("reelping-nonexistent-" + time.Now().Format(time.RFC3339Nano))
	return h
}
