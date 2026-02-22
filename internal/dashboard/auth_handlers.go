package dashboard

import (
	"net/http"
	"strings"
	"time"
)

func (a *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	renderTempl(w, r, http.StatusOK, LoginPage(""))
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderTempl(w, r, http.StatusBadRequest, LoginPage("invalid login request"))
		return
	}
	email := strings.TrimSpace(r.Form.Get("email"))
	password := r.Form.Get("password")
	u, err := a.authenticate(email, password)
	if err != nil {
		renderTempl(w, r, http.StatusUnauthorized, LoginPage("invalid credentials"))
		return
	}
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
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
