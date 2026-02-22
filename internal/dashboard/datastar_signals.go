package dashboard

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/starfederation/datastar-go/datastar"
)

type addServerSignals struct {
	ServerName  string `json:"serverName"`
	ServerURL   string `json:"serverURL"`
	ServerPort  string `json:"serverPort"`
	ServerToken string `json:"serverToken"`
}

type parkDomainSignals struct {
	Domain        string `json:"domain"`
	RecordIP      string `json:"recordIP"`
	RecordType    string `json:"recordType"`
	RecordZone    string `json:"recordZone"`
	RecordTarget  string `json:"recordTarget"`
	RecordText    string `json:"recordText"`
	DomainAccount string `json:"domainAccount"`
	RecordTTL     string `json:"recordTTL"`
}

type transferDomainSignals struct {
	TransferDomain    string `json:"transferDomain"`
	TransferToAccount string `json:"transferToAccount"`
}

type createUserSignals struct {
	NewUserEmail    string `json:"newUserEmail"`
	NewUserPassword string `json:"newUserPassword"`
	NewUserRole     string `json:"newUserRole"`
}

func readDatastarSignals[T any](r *http.Request, out *T) error {
	return datastar.ReadSignals(r, out)
}

func trim(s string) string {
	return strings.TrimSpace(s)
}

func parseIntOrDefault(s string, def int) int {
	s = trim(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
