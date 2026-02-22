package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeDNSNode struct {
	t      *testing.T
	token  string
	mu     sync.Mutex
	record map[string]dnsRecord

	server *httptest.Server
}

func newFakeDNSNode(t *testing.T, token string, seed []dnsRecord) *fakeDNSNode {
	t.Helper()

	n := &fakeDNSNode{
		t:      t,
		token:  token,
		record: make(map[string]dnsRecord),
	}
	for _, r := range seed {
		r = normalizeRecord(r)
		n.record[recordKey(r)] = r
	}

	n.server = httptest.NewServer(http.HandlerFunc(n.handle))
	t.Cleanup(n.server.Close)
	return n
}

func (n *fakeDNSNode) handle(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/v1/") {
		wantAuth := "Bearer " + n.token
		if r.Header.Get("Authorization") != wantAuth || r.Header.Get("X-API-Token") != n.token {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(apiError{Error: "unauthorized"})
			return
		}
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/records":
		n.mu.Lock()
		recs := make([]dnsRecord, 0, len(n.record))
		for _, rec := range n.record {
			recs = append(recs, rec)
		}
		n.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(recordListResponse{Records: recs})
		return

	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/records/") && strings.HasSuffix(r.URL.Path, "/add"):
		name := strings.TrimPrefix(r.URL.Path, "/v1/records/")
		name = strings.TrimSuffix(name, "/add")
		name, _ = url.PathUnescape(name)

		var in recordWriteRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(apiError{Error: "invalid body"})
			return
		}

		rec := dnsRecord{Name: name}
		if in.Type != nil {
			rec.Type = *in.Type
		}
		if in.IP != nil {
			rec.IP = *in.IP
		}
		if in.Text != nil {
			rec.Text = *in.Text
		}
		if in.Target != nil {
			rec.Target = *in.Target
		}
		if in.Priority != nil {
			rec.Priority = *in.Priority
		}
		if in.TTL != nil {
			rec.TTL = *in.TTL
		}
		if in.Zone != nil {
			rec.Zone = *in.Zone
		}

		rec = normalizeRecord(rec)

		n.mu.Lock()
		n.record[recordKey(rec)] = rec
		n.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rec)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (n *fakeDNSNode) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.record)
}

func portFromURL(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return p
}

func TestSyncOnce_ReconcilesDashboardAndServers(t *testing.T) {
	t.Parallel()

	const tok1 = "tok-node-1"
	const tok2 = "tok-node-2"

	n1 := newFakeDNSNode(t, tok1, []dnsRecord{{Name: "a.example.com", Type: "A", IP: "198.51.100.1", TTL: 60, Zone: "example.com"}})
	n2 := newFakeDNSNode(t, tok2, []dnsRecord{{Name: "b.example.com", Type: "A", IP: "198.51.100.2", TTL: 60, Zone: "example.com"}})

	a := New(&http.Client{Timeout: 3 * time.Second}, time.Second, false)
	a.nodes["n1"] = node{ID: "n1", Name: "n1", URL: n1.server.URL, Port: portFromURL(t, n1.server.URL), Token: tok1}
	a.nodes["n2"] = node{ID: "n2", Name: "n2", URL: n2.server.URL, Port: portFromURL(t, n2.server.URL), Token: tok2}

	a.syncOnce(context.Background())

	if got := len(a.dashboard); got != 2 {
		t.Fatalf("dashboard records = %d, want 2", got)
	}
	if got := n1.count(); got != 2 {
		t.Fatalf("node1 records = %d, want 2", got)
	}
	if got := n2.count(); got != 2 {
		t.Fatalf("node2 records = %d, want 2", got)
	}

	if !a.nodes["n1"].Online || !a.nodes["n2"].Online {
		t.Fatalf("expected both nodes online after successful sync")
	}
}
