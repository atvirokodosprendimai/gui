package dashboard

import (
	"context"
	"io"
	"net/http"
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
		RecordsFragment(a.filteredRecordRows("", currentUser(r))),
	)
	return writeDatastarPatchComponent(w, r.Context(), component)
}

func (a *App) writeClockPatch(w io.Writer, r *http.Request, now time.Time) error {
	return writeDatastarPatchComponent(w, r.Context(), ClockFragment(clockText(now)))
}

func writeDatastarPatchComponent(w io.Writer, ctx context.Context, c templ.Component) error {
	if _, err := io.WriteString(w, "event: datastar-patch-elements\n"); err != nil {
		return err
	}
	pw := &datastarElementsWriter{dst: w, atLineStart: true}
	if err := c.Render(ctx, pw); err != nil {
		return err
	}
	return pw.finish()
}

func writeDatastarPatchElements(w io.Writer, elements string) error {
	return writeDatastarPatchComponent(w, context.Background(), templ.Raw(elements))
}

type datastarElementsWriter struct {
	dst         io.Writer
	atLineStart bool
}

func (w *datastarElementsWriter) Write(p []byte) (int, error) {
	for i, b := range p {
		if w.atLineStart {
			if _, err := io.WriteString(w.dst, "data: elements "); err != nil {
				return i, err
			}
			w.atLineStart = false
		}
		if _, err := w.dst.Write([]byte{b}); err != nil {
			return i, err
		}
		if b == '\n' {
			w.atLineStart = true
		}
	}
	return len(p), nil
}

func (w *datastarElementsWriter) finish() error {
	if w.atLineStart {
		_, err := io.WriteString(w.dst, "\n")
		return err
	}
	_, err := io.WriteString(w.dst, "\n\n")
	return err
}
