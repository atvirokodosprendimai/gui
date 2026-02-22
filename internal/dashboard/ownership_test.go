package dashboard

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFilteredRecordRows_IncludeAccount(t *testing.T) {
	t.Parallel()

	a := New(nil, 0)
	a.dashboard[recordKey(dnsRecord{Name: "app.example.com", Type: "A", IP: "198.51.100.10", Zone: "example.com", TTL: 60})] = normalizeRecord(dnsRecord{Name: "app.example.com", Type: "A", IP: "198.51.100.10", Zone: "example.com", TTL: 60})
	a.domainOwners["app.example.com."] = "account-a"

	rows := a.filteredRecordRows("account-a")
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Account != "account-a" {
		t.Fatalf("row account = %q, want account-a", rows[0].Account)
	}
}

func TestHandleTransferDomain(t *testing.T) {
	t.Parallel()

	a := New(nil, 0)
	r := a.Routes()

	q := url.Values{}
	q.Set("domain", "app.example.com")
	q.Set("to_account", "account-b")

	req := httptest.NewRequest(http.MethodGet, "/ui/domain/transfer?"+q.Encode(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if got := a.domainOwners["app.example.com."]; got != "account-b" {
		t.Fatalf("domain owner = %q, want account-b", got)
	}
	if got := a.consumeFlash(); got != "transferred app.example.com. to account account-b" {
		t.Fatalf("flash = %q", got)
	}
}
