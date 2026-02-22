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

func TestValidateDNSRecordInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rec     dnsRecord
		wantErr bool
	}{
		{name: "valid A", rec: dnsRecord{Name: "example.com", Type: "A", IP: "198.51.100.10", TTL: 60}},
		{name: "invalid A", rec: dnsRecord{Name: "example.com", Type: "A", IP: "bad"}, wantErr: true},
		{name: "invalid AAAA", rec: dnsRecord{Name: "example.com", Type: "AAAA", IP: "198.51.100.10"}, wantErr: true},
		{name: "valid TXT", rec: dnsRecord{Name: "example.com", Type: "TXT", Text: "hello"}},
		{name: "missing TXT", rec: dnsRecord{Name: "example.com", Type: "TXT"}, wantErr: true},
		{name: "valid CNAME", rec: dnsRecord{Name: "example.com", Type: "CNAME", Target: "target.example.net"}},
		{name: "missing CNAME target", rec: dnsRecord{Name: "example.com", Type: "CNAME"}, wantErr: true},
		{name: "unsupported type", rec: dnsRecord{Name: "example.com", Type: "SRV"}, wantErr: true},
		{name: "ttl too high", rec: dnsRecord{Name: "example.com", Type: "A", IP: "198.51.100.10", TTL: maxTTL + 1}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateDNSRecordInput(tt.rec)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
