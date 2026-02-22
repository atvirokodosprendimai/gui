package dashboard

import "testing"

func TestNormalizeFQDN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "trim and lowercase", in: " Example.COM ", want: "example.com."},
		{name: "already normalized", in: "api.example.com.", want: "api.example.com."},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeFQDN(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeFQDN(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRecordKey_NormalizesFields(t *testing.T) {
	t.Parallel()

	a := dnsRecord{Name: "WWW.EXAMPLE.COM", Type: "a", IP: "198.51.100.10"}
	b := dnsRecord{Name: "www.example.com.", Type: "A", IP: "198.51.100.10"}

	if recordKey(a) != recordKey(b) {
		t.Fatalf("expected equivalent record keys, got %q and %q", recordKey(a), recordKey(b))
	}
}

func TestNodeEndpoint(t *testing.T) {
	t.Parallel()

	n := node{URL: "10.0.0.3", Port: 8081}
	if got, want := n.endpoint(), "http://10.0.0.3:8081"; got != want {
		t.Fatalf("endpoint() = %q, want %q", got, want)
	}

	n = node{URL: "https://dns.internal", Port: 8443}
	if got, want := n.endpoint(), "https://dns.internal:8443"; got != want {
		t.Fatalf("endpoint() = %q, want %q", got, want)
	}
}
