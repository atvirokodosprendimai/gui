package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilteredRecordRows_IncludeAccount(t *testing.T) {
	t.Parallel()

	a := New(nil, 0, false)
	a.dashboard[recordKey(dnsRecord{Name: "app.example.com", Type: "A", IP: "198.51.100.10", Zone: "example.com", TTL: 60})] = normalizeRecord(dnsRecord{Name: "app.example.com", Type: "A", IP: "198.51.100.10", Zone: "example.com", TTL: 60})
	a.domainOwners["app.example.com."] = "account-a"

	rows := a.filteredRecordRows("account-a", nil)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Account != "account-a" {
		t.Fatalf("row account = %q, want account-a", rows[0].Account)
	}
}

func TestHandleTransferDomain(t *testing.T) {
	t.Parallel()

	a := New(nil, 0, false)
	body := `{"transferDomain":"app.example.com","transferToAccount":"account-b"}`

	a.domainOwners["app.example.com."] = "admin@local"

	req := httptest.NewRequest(http.MethodPost, "/ui/domain/transfer", strings.NewReader(body)).WithContext(
		context.WithValue(context.Background(), userContextKey{}, &User{Email: "admin@local", Role: roleAdmin}),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	a.handleTransferDomain(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if got := a.domainOwners["app.example.com."]; got != "account-b" {
		t.Fatalf("domain owner = %q, want account-b", got)
	}
}
