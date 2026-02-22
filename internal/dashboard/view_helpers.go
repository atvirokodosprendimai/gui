package dashboard

import (
	"net/url"
	"strings"
	"time"
)

func serverStatusText(n node) string {
	status := "offline"
	if n.Online {
		status = "online"
	}
	if n.LastError == "" {
		return status
	}
	return status + " - " + n.LastError
}

func serverLastSyncText(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}

func deleteServerAction(id string) string {
	return "@post('/ui/server/delete/" + url.PathEscape(id) + "')"
}

func recordValueText(rec dnsRecord) string {
	return recordValue(rec)
}

func clockText(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}

func recordSearchIndex(row recordRow) string {
	return strings.ToLower(
		row.Record.Name + " " + row.Record.Type + " " + recordValueText(row.Record) + " " + row.Record.Zone + " " + row.Account,
	)
}
