package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (a *App) fetchRecords(ctx context.Context, n node) ([]dnsRecord, error) {
	endpoint := n.endpoint()
	if endpoint == "" {
		return nil, fmt.Errorf("invalid server endpoint")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/v1/records", nil)
	if err != nil {
		return nil, err
	}
	setAuth(req, n.Token)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusMultipleChoices-1 {
		return nil, decodeHTTPError(resp)
	}

	var out recordListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out.Records, nil
}

func (a *App) addRecordToServer(ctx context.Context, n node, rec dnsRecord) error {
	endpoint := n.endpoint()
	if endpoint == "" {
		return fmt.Errorf("invalid server endpoint")
	}

	name := strings.TrimSuffix(normalizeFQDN(rec.Name), ".")
	body, err := json.Marshal(buildRecordWrite(rec))
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint+"/v1/records/"+url.PathEscape(name)+"/add",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	setAuth(req, n.Token)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusMultipleChoices-1 {
		return decodeHTTPError(resp)
	}

	return nil
}

func buildRecordWrite(rec dnsRecord) recordWriteRequest {
	rec = normalizeRecord(rec)
	var req recordWriteRequest

	if rec.IP != "" {
		v := rec.IP
		req.IP = &v
	}
	if rec.Type != "" {
		v := rec.Type
		req.Type = &v
	}
	if rec.Text != "" {
		v := rec.Text
		req.Text = &v
	}
	if rec.Target != "" {
		v := strings.TrimSuffix(rec.Target, ".")
		req.Target = &v
	}
	if rec.Priority > 0 {
		v := rec.Priority
		req.Priority = &v
	}
	if rec.TTL > 0 {
		v := rec.TTL
		req.TTL = &v
	}
	if rec.Zone != "" {
		v := strings.TrimSuffix(rec.Zone, ".")
		req.Zone = &v
	}

	return req
}

func setAuth(req *http.Request, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-API-Token", token)
}

func decodeHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))

	var e apiError
	if err := json.Unmarshal(body, &e); err == nil && strings.TrimSpace(e.Error) != "" {
		return fmt.Errorf("upstream %d: %s", resp.StatusCode, e.Error)
	}

	t := strings.TrimSpace(string(body))
	if t == "" {
		t = http.StatusText(resp.StatusCode)
	}

	return fmt.Errorf("upstream %d: %s", resp.StatusCode, t)
}
