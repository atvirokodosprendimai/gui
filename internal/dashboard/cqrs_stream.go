package dashboard

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

func (a *App) handleCQRSStream(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	watcher := a.addWatcher()
	defer a.removeWatcher(watcher)

	if err := a.patchElement(sse, r, "flash", false); err != nil {
		return
	}
	if err := a.patchElement(sse, r, "overview", false); err != nil {
		return
	}
	if err := a.patchElement(sse, r, "servers", false); err != nil {
		return
	}
	if err := a.patchElement(sse, r, "records", false); err != nil {
		return
	}
	if err := a.patchElement(sse, r, "users", false); err != nil {
		return
	}
	if err := a.patchClock(sse, time.Now().UTC()); err != nil {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	clientSessionToken := sessionTokenFromRequest(r)
	viewer := currentUser(r)
	clientUserID := ""
	if viewer != nil {
		clientUserID = strconv.FormatUint(uint64(viewer.ID), 10)
	}

	for {
		select {
		case <-a.lifecycleCtx.Done():
			return
		case <-sse.Context().Done():
			return
		case upd := <-watcher:
			receivedAt := time.Now().UTC()
			if upd.Subject != "" && upd.Subject != subjectFEUpdate {
				continue
			}
			switch upd.Scope {
			case scopeSession:
				if upd.SessionID == "" {
					continue
				}
				if upd.SessionID != "" && upd.SessionID != clientSessionToken {
					continue
				}
			case scopeUser:
				if upd.UserID == "" {
					continue
				}
				if upd.UserID != "" && upd.UserID != clientUserID {
					continue
				}
			}
			if upd.Scope == scopeSession && clientSessionToken == "" {
				continue
			}
			if err := a.patchElement(sse, r, upd.Element, true); err != nil {
				return
			}
			if upd.EmittedAt > 0 {
				lag := receivedAt.Sub(time.UnixMilli(upd.EmittedAt))
				if lag >= 0 {
					log.Printf("cqrs patch element=%s scope=%s lag_ms=%d", upd.Element, upd.Scope, lag.Milliseconds())
				}
			}
		case now := <-ticker.C:
			if err := a.patchClock(sse, now.UTC()); err != nil {
				return
			}
		}
	}
}

func (a *App) patchElement(sse *datastar.ServerSentEventGenerator, r *http.Request, el string, consumeFlash bool) error {
	viewer := currentUser(r)
	switch el {
	case "flash":
		msg := ""
		if consumeFlash {
			msg = a.consumeFlash(r)
		}
		return sse.PatchElementTempl(FlashFragment(msg))
	case "overview":
		nodeCount, onlineCount, recordCount := a.overviewCounts()
		return sse.PatchElementTempl(OverviewFragment(nodeCount, onlineCount, recordCount))
	case "servers":
		return sse.PatchElementTempl(ServersFragment(a.sortedNodes()))
	case "records":
		return sse.PatchElementTempl(RecordsFragment(a.filteredRecordRows("", viewer)))
	case "users":
		return sse.PatchElementTempl(UsersFragment(viewer, a.sortedUsers()))
	default:
		return nil
	}
}

func (a *App) patchClock(sse *datastar.ServerSentEventGenerator, now time.Time) error {
	return sse.PatchElementTempl(ClockFragment(clockText(now)))
}
