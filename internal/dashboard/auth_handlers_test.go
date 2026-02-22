package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleLogin_RejectsOversizedBody(t *testing.T) {
	t.Parallel()

	a := New(nil, 0, false)
	body := "email=a@example.com&password=" + strings.Repeat("x", int(maxLoginBodyBytes)+64)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	a.handleLogin(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
