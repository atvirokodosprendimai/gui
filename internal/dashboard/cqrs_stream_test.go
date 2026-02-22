package dashboard

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteDatastarPatchElements(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	err := writeDatastarPatchElements(&b, `<div id="clock">2026-02-22T12:00:00Z</div>`)
	if err != nil {
		t.Fatalf("writeDatastarPatchElements returned error: %v", err)
	}

	got := b.String()
	if !strings.Contains(got, "event: datastar-patch-elements\n") {
		t.Fatalf("missing datastar SSE event header: %q", got)
	}
	if !strings.Contains(got, "data: elements <div id=\"clock\">2026-02-22T12:00:00Z</div>\n") {
		t.Fatalf("missing patch elements payload: %q", got)
	}
}

func TestHandleCQRSStream_WritesInitialReadModelAndClock(t *testing.T) {
	t.Parallel()

	a := New(nil, time.Second)
	rec := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/any/cqrs", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		a.handleCQRSStream(rec, req)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("CQRS stream handler did not stop after context cancellation")
	}

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Fatalf("expected datastar patch events in body, got %q", body)
	}
	if !strings.Contains(body, `id="overview"`) {
		t.Fatalf("expected overview patch in stream body, got %q", body)
	}
	if !strings.Contains(body, `id="clock"`) {
		t.Fatalf("expected clock patch in stream body, got %q", body)
	}
}
