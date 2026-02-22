package dashboard

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
)

func (a *App) handleCQRSStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	watcher := a.addWatcher()
	defer a.removeWatcher(watcher)

	if err := a.writeReadModelPatch(w, r, false); err != nil {
		return
	}
	if err := a.writeClockPatch(w, r, time.Now().UTC()); err != nil {
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-watcher:
			if err := a.writeReadModelPatch(w, r, true); err != nil {
				return
			}
			flusher.Flush()
		case now := <-ticker.C:
			if err := a.writeClockPatch(w, r, now.UTC()); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (a *App) writeReadModelPatch(w io.Writer, r *http.Request, consumeFlash bool) error {
	flash := ""
	if consumeFlash {
		flash = a.consumeFlash()
	}
	nodeCount, onlineCount, recordCount := a.overviewCounts()
	component := templ.Join(
		FlashFragment(flash),
		OverviewFragment(nodeCount, onlineCount, recordCount),
		ServersFragment(a.sortedNodes()),
		RecordsFragment(a.filteredRecordRows("")),
	)
	html, err := renderComponentToString(r, component)
	if err != nil {
		return err
	}
	return writeDatastarPatchElements(w, html)
}

func (a *App) writeClockPatch(w io.Writer, r *http.Request, now time.Time) error {
	html, err := renderComponentToString(r, ClockFragment(clockText(now)))
	if err != nil {
		return err
	}
	return writeDatastarPatchElements(w, html)
}

func writeDatastarPatchElements(w io.Writer, elements string) error {
	lines := strings.Split(elements, "\n")
	if _, err := io.WriteString(w, "event: datastar-patch-elements\n"); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "data: elements %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}
