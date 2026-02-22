package dashboard

import (
	"net/http/httptest"
	"testing"
)

func TestLoginLimiter_BlocksByIPAcrossEmails(t *testing.T) {
	t.Parallel()

	a := New(nil, 0, false)
	req := httptest.NewRequest("POST", "/auth/login", nil)
	req.RemoteAddr = "203.0.113.10:12345"

	ipKey := a.loginIPKey(req)
	acctB := a.loginKey(req, "b@example.com")

	for i := 0; i < 5; i++ {
		a.noteLoginFail(ipKey)
	}

	if a.allowLogin(ipKey) {
		t.Fatal("expected IP key to be blocked")
	}
	if a.allowLogin(ipKey) && a.allowLogin(acctB) {
		t.Fatal("expected combined IP+account guard to block login")
	}
}
