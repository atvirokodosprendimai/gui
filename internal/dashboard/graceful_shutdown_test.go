package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunSyncLoop_StopsOnCancelWithInFlightRequest(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	var startedOnce sync.Once

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/records" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		startedOnce.Do(func() { close(started) })
		<-r.Context().Done()
	}))
	defer srv.Close()

	a := New(&http.Client{Timeout: 30 * time.Second}, 10*time.Millisecond, false)
	a.nodes["n1"] = node{ID: "n1", Name: "n1", URL: srv.URL, Port: portFromURL(t, srv.URL), Token: "tok"}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.RunSyncLoop(ctx)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for in-flight sync request")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sync loop did not stop after cancellation")
	}
}

func TestSyncOnce_SkipsNetworkWhenContextCanceled(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := New(&http.Client{Timeout: 30 * time.Second}, time.Second, false)
	a.nodes["n1"] = node{ID: "n1", Name: "n1", URL: srv.URL, Port: portFromURL(t, srv.URL), Token: "tok"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a.syncOnce(ctx)

	if got := hits.Load(); got != 0 {
		t.Fatalf("unexpected outbound requests: got %d, want 0", got)
	}
}
