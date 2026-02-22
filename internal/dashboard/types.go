package dashboard

import (
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxTTL = 604800

type dnsRecord struct {
	ID       int64     `json:"id,omitempty"`
	Name     string    `json:"name"`
	Type     string    `json:"type,omitempty"`
	IP       string    `json:"ip,omitempty"`
	Text     string    `json:"text,omitempty"`
	Target   string    `json:"target,omitempty"`
	Priority int       `json:"priority,omitempty"`
	TTL      int       `json:"ttl,omitempty"`
	Zone     string    `json:"zone,omitempty"`
	Updated  time.Time `json:"updated_at,omitempty"`
	Version  int64     `json:"version,omitempty"`
	Source   string    `json:"source,omitempty"`
}

type recordRow struct {
	Record  dnsRecord
	Account string
}

type recordListResponse struct {
	Records []dnsRecord `json:"records"`
}

type apiError struct {
	Error string `json:"error"`
}

type recordWriteRequest struct {
	IP        *string `json:"ip,omitempty"`
	Type      *string `json:"type,omitempty"`
	Text      *string `json:"text,omitempty"`
	Target    *string `json:"target,omitempty"`
	Priority  *int    `json:"priority,omitempty"`
	TTL       *int    `json:"ttl,omitempty"`
	Zone      *string `json:"zone,omitempty"`
	Propagate *bool   `json:"propagate,omitempty"`
}

type node struct {
	ID          string
	Name        string
	URL         string
	Port        int
	Token       string
	LastSyncAt  time.Time
	LastError   string
	Online      bool
	RecordCount int
}

func (n node) endpoint() string {
	raw := strings.TrimSpace(n.URL)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	host := u.Hostname()
	if host == "" {
		return ""
	}

	port := n.Port
	if port <= 0 {
		port = 8080
	}

	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

func normalizeRecord(rec dnsRecord) dnsRecord {
	rec.Name = normalizeFQDN(rec.Name)
	rec.Zone = normalizeFQDN(rec.Zone)
	rec.Type = strings.ToUpper(strings.TrimSpace(rec.Type))
	if rec.Target != "" {
		rec.Target = normalizeFQDN(rec.Target)
	}
	if rec.TTL < 0 {
		rec.TTL = 0
	}
	return rec
}

func normalizeFQDN(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	s = strings.TrimSuffix(s, ".")
	return s + "."
}

func recordValue(r dnsRecord) string {
	switch strings.ToUpper(r.Type) {
	case "TXT":
		return r.Text
	case "CNAME":
		return r.Target
	case "MX":
		if r.Priority > 0 {
			return fmt.Sprintf("%d %s", r.Priority, r.Target)
		}
		return r.Target
	default:
		return r.IP
	}
}

func recordKey(r dnsRecord) string {
	r = normalizeRecord(r)
	return r.Name + "|" + r.Type + "|" + recordValue(r)
}

func nodeID(name, host string, port int) string {
	h := net.JoinHostPort(strings.ToLower(strings.TrimSpace(host)), strconv.Itoa(port))
	return strings.ToLower(strings.TrimSpace(name)) + "|" + h
}

func esc(s string) string {
	return template.HTMLEscapeString(s)
}

func validateDNSRecordInput(rec dnsRecord) error {
	rec = normalizeRecord(rec)
	if rec.Name == "" {
		return errors.New("domain is required")
	}
	if rec.TTL < 0 || rec.TTL > maxTTL {
		return fmt.Errorf("ttl must be between 0 and %d", maxTTL)
	}
	switch rec.Type {
	case "A":
		ip := net.ParseIP(strings.TrimSpace(rec.IP))
		if ip == nil || ip.To4() == nil {
			return errors.New("A record requires a valid IPv4 address")
		}
	case "AAAA":
		ip := net.ParseIP(strings.TrimSpace(rec.IP))
		if ip == nil || ip.To4() != nil {
			return errors.New("AAAA record requires a valid IPv6 address")
		}
	case "TXT":
		if strings.TrimSpace(rec.Text) == "" {
			return errors.New("TXT record requires text value")
		}
	case "CNAME":
		if normalizeFQDN(rec.Target) == "" {
			return errors.New("CNAME record requires target")
		}
	case "MX":
		if normalizeFQDN(rec.Target) == "" {
			return errors.New("MX record requires target")
		}
	default:
		return errors.New("unsupported record type")
	}
	if rec.Zone != "" && normalizeFQDN(rec.Zone) == "" {
		return errors.New("invalid zone")
	}
	return nil
}
