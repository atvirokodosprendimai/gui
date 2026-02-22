package dashboard

import "testing"

func TestScopeAndIDFromSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		subject string
		scope   string
		id      string
		ok      bool
	}{
		{subject: "fe.update.global", scope: scopeGlobal, id: "", ok: true},
		{subject: "fe.update.user.42", scope: scopeUser, id: "42", ok: true},
		{subject: "fe.update.session.abcd", scope: scopeSession, id: "abcd", ok: true},
		{subject: "fe.update.user.", ok: false},
		{subject: "fe.update.user.a.b", ok: false},
		{subject: "fe.update", ok: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.subject, func(t *testing.T) {
			t.Parallel()
			scope, id, ok := scopeAndIDFromSubject(tt.subject)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if scope != tt.scope {
				t.Fatalf("scope = %q, want %q", scope, tt.scope)
			}
			if id != tt.id {
				t.Fatalf("id = %q, want %q", id, tt.id)
			}
		})
	}
}

func TestSubjectMatchesUpdate(t *testing.T) {
	t.Parallel()

	if !subjectMatchesUpdate(scopeUser, "42", uiUpdate{Scope: scopeUser, UserID: "42"}) {
		t.Fatal("expected user scope update to match")
	}
	if subjectMatchesUpdate(scopeUser, "42", uiUpdate{Scope: scopeUser, UserID: "7"}) {
		t.Fatal("expected mismatched user scope update to be rejected")
	}
	if !subjectMatchesUpdate(scopeSession, "tok_1", uiUpdate{Scope: scopeSession, SessionID: "tok_1"}) {
		t.Fatal("expected session scope update to match")
	}
	if subjectMatchesUpdate(scopeGlobal, "", uiUpdate{Scope: scopeGlobal, UserID: "42"}) {
		t.Fatal("expected global update with identity fields to be rejected")
	}
}

func TestSanitizeSubjectToken(t *testing.T) {
	t.Parallel()

	if got, want := sanitizeSubjectToken(" user.*.a>b\n "), "user___a_b"; got != want {
		t.Fatalf("sanitizeSubjectToken() = %q, want %q", got, want)
	}
}
