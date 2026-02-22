package dashboard

import (
	"net/http"
	"strings"
	"time"
)

const maxLoginBodyBytes int64 = 16 << 10

func (a *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	renderTempl(w, r, http.StatusOK, LoginPage(""))
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)
	if err := r.ParseForm(); err != nil {
		renderTempl(w, r, http.StatusBadRequest, LoginPage("invalid login request"))
		return
	}
	email := strings.TrimSpace(r.Form.Get("email"))
	password := r.Form.Get("password")
	key := a.loginKey(r, email)
	ipKey := a.loginIPKey(r)
	if !a.allowLogin(ipKey) || !a.allowLogin(key) {
		renderTempl(w, r, http.StatusTooManyRequests, LoginPage("too many attempts, retry later"))
		return
	}
	u, err := a.authenticate(email, password)
	if err != nil {
		a.noteLoginFail(key)
		a.noteLoginFail(ipKey)
		renderTempl(w, r, http.StatusUnauthorized, LoginPage("invalid credentials"))
		return
	}
	a.noteLoginSuccess(key)
	a.noteLoginSuccess(ipKey)
	token, err := a.createSession(u.ID)
	if err != nil {
		renderTempl(w, r, http.StatusInternalServerError, LoginPage("session creation failed"))
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().UTC().Add(24 * time.Hour),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session_token"); err == nil {
		a.deleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.requestIsHTTPS(r),
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if !a.trustProxy {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}
