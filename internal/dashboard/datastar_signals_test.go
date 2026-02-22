package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadDatastarSignals_CreateUserJSON(t *testing.T) {
	t.Parallel()

	body := `{"newUserEmail":"admin@example.com","newUserPassword":"secret","newUserRole":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/ui/users/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var sig createUserSignals
	if err := readDatastarSignals(req, &sig); err != nil {
		t.Fatalf("readDatastarSignals failed: %v", err)
	}

	if sig.NewUserEmail != "admin@example.com" || sig.NewUserPassword != "secret" || sig.NewUserRole != "admin" {
		t.Fatalf("unexpected parsed signals: %#v", sig)
	}
}

func TestParseIntOrDefault(t *testing.T) {
	t.Parallel()

	if got := parseIntOrDefault("8081", 8080); got != 8081 {
		t.Fatalf("got %d, want 8081", got)
	}
	if got := parseIntOrDefault("", 8080); got != 8080 {
		t.Fatalf("got %d, want 8080", got)
	}
	if got := parseIntOrDefault("bad", 8080); got != 8080 {
		t.Fatalf("got %d, want 8080", got)
	}
}
