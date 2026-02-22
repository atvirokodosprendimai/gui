package dashboard

import (
	"net/http"
	"strings"
	"time"
)

type loginState struct {
	Fails        int
	BlockedUntil time.Time
	LastSeen     time.Time
}

func loginKey(r *http.Request, email string) string {
	ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if ip == "" {
		ip = r.RemoteAddr
	}
	return strings.ToLower(strings.TrimSpace(email)) + "|" + ip
}

func (a *App) allowLogin(key string) bool {
	now := time.Now().UTC()
	a.authMu.Lock()
	defer a.authMu.Unlock()

	a.cleanupLoginLimiter(now)
	st := a.loginLimiter[key]
	return !now.Before(st.BlockedUntil)
}

func (a *App) noteLoginFail(key string) {
	now := time.Now().UTC()
	a.authMu.Lock()
	defer a.authMu.Unlock()

	st := a.loginLimiter[key]
	st.Fails++
	st.LastSeen = now
	if st.Fails >= 5 {
		st.BlockedUntil = now.Add(5 * time.Minute)
	}
	a.loginLimiter[key] = st
}

func (a *App) noteLoginSuccess(key string) {
	a.authMu.Lock()
	defer a.authMu.Unlock()
	delete(a.loginLimiter, key)
}

func (a *App) cleanupLoginLimiter(now time.Time) {
	for key, st := range a.loginLimiter {
		if now.Sub(st.LastSeen) > 24*time.Hour {
			delete(a.loginLimiter, key)
		}
	}
}
