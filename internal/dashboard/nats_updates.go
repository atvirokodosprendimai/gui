package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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

func (b *natsBridge) Close(ctx context.Context) {
	if b == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, s := range b.subs {
		if s != nil {
			done := make(chan struct{})
			go func(sub *nats.Subscription) {
				_ = sub.Drain()
				close(done)
			}(s)
			select {
			case <-done:
			case <-ctx.Done():
				_ = s.Unsubscribe()
			}
		}
	}
	if b.nc != nil {
		done := make(chan struct{})
		go func() {
			b.nc.Drain()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
		}
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
		scope, subjectID, ok := scopeAndIDFromSubject(m.Subject)
		if !ok {
			log.Printf("dropping nats ui update: invalid subject %q", m.Subject)
			return
		}
		var upd uiUpdate
		if err := json.Unmarshal(m.Data, &upd); err != nil {
			log.Printf("dropping nats ui update: decode failed for %q: %v", m.Subject, err)
			return
		}
		if upd.Subject == "" {
			upd.Subject = subjectFEUpdate
		}
		if upd.Scope == "" {
			upd.Scope = scope
		}
		if upd.Scope == scopeUser && strings.TrimSpace(upd.UserID) == "" {
			upd.UserID = subjectID
		}
		if upd.Scope == scopeSession && strings.TrimSpace(upd.SessionID) == "" {
			upd.SessionID = subjectID
		}
		if !subjectMatchesUpdate(scope, subjectID, upd) {
			log.Printf("dropping nats ui update: scope/payload mismatch subject=%q payload_scope=%q", m.Subject, upd.Scope)
			return
		}
		if strings.TrimSpace(upd.Element) == "" {
			log.Printf("dropping nats ui update: empty element subject=%q", m.Subject)
			return
		}
		if !isValidScopedUpdate(upd) {
			log.Printf("dropping nats ui update: invalid scoped payload subject=%q", m.Subject)
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
	if upd.EmittedAt == 0 {
		upd.EmittedAt = time.Now().UTC().UnixMilli()
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

func scopeAndIDFromSubject(subject string) (scope string, id string, ok bool) {
	switch {
	case subject == subjectFEUpdateGlobal:
		return scopeGlobal, "", true
	case strings.HasPrefix(subject, "fe.update.user."):
		id = strings.TrimSpace(strings.TrimPrefix(subject, "fe.update.user."))
		if id == "" || strings.Contains(id, ".") {
			return "", "", false
		}
		return scopeUser, id, true
	case strings.HasPrefix(subject, "fe.update.session."):
		id = strings.TrimSpace(strings.TrimPrefix(subject, "fe.update.session."))
		if id == "" || strings.Contains(id, ".") {
			return "", "", false
		}
		return scopeSession, id, true
	default:
		return "", "", false
	}
}

func subjectMatchesUpdate(scope string, subjectID string, upd uiUpdate) bool {
	if upd.Scope != scope {
		return false
	}
	switch scope {
	case scopeGlobal:
		return strings.TrimSpace(upd.UserID) == "" && strings.TrimSpace(upd.SessionID) == ""
	case scopeUser:
		if strings.TrimSpace(upd.UserID) == "" {
			return false
		}
		return sanitizeSubjectToken(upd.UserID) == subjectID
	case scopeSession:
		if strings.TrimSpace(upd.SessionID) == "" {
			return false
		}
		return sanitizeSubjectToken(upd.SessionID) == subjectID
	default:
		return false
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
	if v == "" {
		return ""
	}
	b := strings.Builder{}
	b.Grow(len(v))
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
