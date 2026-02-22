package dashboard

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSetAuth(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	setAuth(req, "token-123")

	if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("X-API-Token"); got != "token-123" {
		t.Fatalf("X-API-Token = %q", got)
	}
}

func TestDecodeHTTPError_JSONModel(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
	}

	err := decodeHTTPError(resp)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "upstream 401: unauthorized" {
		t.Fatalf("error = %q", got)
	}
}

func TestBuildRecordWrite_OmitsUnsetFields(t *testing.T) {
	t.Parallel()

	req := buildRecordWrite(dnsRecord{Name: "a.example.com", Type: "A", IP: "198.51.100.50"})

	if req.IP == nil || *req.IP != "198.51.100.50" {
		t.Fatalf("expected IP to be set")
	}
	if req.Type == nil || *req.Type != "A" {
		t.Fatalf("expected type to be set")
	}
	if req.TTL != nil {
		t.Fatalf("expected TTL nil when unset")
	}
	if req.Zone != nil {
		t.Fatalf("expected zone nil when unset")
	}
	if req.Propagate != nil {
		t.Fatalf("expected propagate nil when unset")
	}
}
