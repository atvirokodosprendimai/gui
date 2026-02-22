package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
)

const (
	scopeGlobal  = "global"
	scopeUser    = "user"
	scopeSession = "session"

	subjectFEUpdateGlobal  = "fe.update.global"
	subjectFEUpdateUser    = "fe.update.user.>"
	subjectFEUpdateSession = "fe.update.session.>"
)

type natsBridge struct {
	nc   *nats.Conn
	subs []*nats.Subscription
}

func (b *natsBridge) Close() {
	if b == nil {
		return
	}
	for _, s := range b.subs {
		if s != nil {
			_ = s.Drain()
		}
	}
	if b.nc != nil {
		b.nc.Drain()
		b.nc.Close()
	}
}

func (a *App) ConfigureNATS(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}

	nc, err := nats.Connect(url, nats.Name("gui-dashboard"))
	if err != nil {
		return err
	}

	h := func(m *nats.Msg) {
		var upd uiUpdate
		if err := json.Unmarshal(m.Data, &upd); err != nil {
			return
		}
		if upd.Subject == "" {
			upd.Subject = subjectFEUpdate
		}
		if upd.Scope == "" {
			upd.Scope = scopeFromSubject(m.Subject)
		}
		if strings.TrimSpace(upd.Element) == "" {
			return
		}
		if !isValidScopedUpdate(upd) {
			return
		}
		a.fanoutUpdate(upd)
	}

	globalSub, err := nc.Subscribe(subjectFEUpdateGlobal, h)
	if err != nil {
		nc.Close()
		return err
	}
	userSub, err := nc.Subscribe(subjectFEUpdateUser, h)
	if err != nil {
		globalSub.Unsubscribe()
		nc.Close()
		return err
	}
	sessionSub, err := nc.Subscribe(subjectFEUpdateSession, h)
	if err != nil {
		globalSub.Unsubscribe()
		userSub.Unsubscribe()
		nc.Close()
		return err
	}

	a.natsBridge = &natsBridge{nc: nc, subs: []*nats.Subscription{globalSub, userSub, sessionSub}}
	return nil
}

func (a *App) emitUpdate(upd uiUpdate) {
	if upd.Subject == "" {
		upd.Subject = subjectFEUpdate
	}
	if upd.Scope == "" {
		upd.Scope = scopeGlobal
	}
	if !isValidScopedUpdate(upd) {
		return
	}

	if a.natsBridge == nil || a.natsBridge.nc == nil {
		a.fanoutUpdate(upd)
		return
	}

	b, err := json.Marshal(upd)
	if err != nil {
		return
	}
	subject := subjectForUpdate(upd)
	if subject == "" {
		return
	}
	_ = a.natsBridge.nc.Publish(subject, b)
}

func (a *App) fanoutUpdate(upd uiUpdate) {
	a.mu.RLock()
	watchers := make([]chan uiUpdate, 0, len(a.watchers))
	for ch := range a.watchers {
		watchers = append(watchers, ch)
	}
	a.mu.RUnlock()

	for _, ch := range watchers {
		select {
		case ch <- upd:
		default:
		}
	}
}

func scopeFromSubject(subject string) string {
	switch {
	case subject == subjectFEUpdateGlobal:
		return scopeGlobal
	case strings.HasPrefix(subject, "fe.update.user."):
		return scopeUser
	case strings.HasPrefix(subject, "fe.update.session."):
		return scopeSession
	default:
		return scopeGlobal
	}
}

func subjectForUpdate(upd uiUpdate) string {
	switch upd.Scope {
	case scopeUser:
		if strings.TrimSpace(upd.UserID) == "" {
			return ""
		}
		return fmt.Sprintf("fe.update.user.%s", sanitizeSubjectToken(upd.UserID))
	case scopeSession:
		if strings.TrimSpace(upd.SessionID) == "" {
			return ""
		}
		return fmt.Sprintf("fe.update.session.%s", sanitizeSubjectToken(upd.SessionID))
	default:
		return subjectFEUpdateGlobal
	}
}

func isValidScopedUpdate(upd uiUpdate) bool {
	switch upd.Scope {
	case "", scopeGlobal:
		return true
	case scopeUser:
		return strings.TrimSpace(upd.UserID) != ""
	case scopeSession:
		return strings.TrimSpace(upd.SessionID) != ""
	default:
		return false
	}
}

func sanitizeSubjectToken(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, ".", "_")
	return v
}
