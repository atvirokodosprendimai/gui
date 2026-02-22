# DNS Server API Contract

This is the integration contract for external clients.

Notes:

- This contract is derived from server code (`http.go`, `types.go`, `util.go`) and intentionally ignores `*.json` files.
- JSON request bodies reject unknown fields (`decodeJSON` uses `DisallowUnknownFields`).
- All names are normalized to lowercase FQDN with trailing dot in persisted/returned objects.

## Base URL and transport

- Base URL example: `http://<host>:8080`
- DNS over HTTPS endpoint: `/dns-query`
- JSON control endpoints: `/v1/*`

## Authentication

- Control API (`/v1/records*`, `/v1/zones*`):
  - `Authorization: Bearer <API_TOKEN>` OR `X-API-Token: <API_TOKEN>`
  - If `API_TOKEN` is empty in server config, control API is open.
- Sync endpoint (`/v1/sync/event`):
  - `X-Sync-Token: <SYNC_TOKEN>`
  - If `SYNC_TOKEN` is empty, endpoint is open.

## Endpoints

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/healthz` | none | Liveness and basic node metadata |
| GET | `/dns-query?dns=<base64url-wire>` | none | DoH GET |
| POST | `/dns-query` | none | DoH POST (`application/dns-message`) |
| GET | `/v1/records` | API token | List records |
| PUT | `/v1/records/{name}` | API token | Upsert/replace RRset for `(name,type)` |
| POST | `/v1/records/{name}/add` | API token | Add one RR member |
| POST | `/v1/records/{name}/remove` | API token | Remove one RR member |
| DELETE | `/v1/records/{name}` | API token | Delete records by name (optionally filtered by type) |
| GET | `/v1/zones` | API token | List zones |
| PUT | `/v1/zones/{zone}` | API token | Upsert zone NS/SOA config |
| POST | `/v1/sync/event` | sync token | Apply replication event |

## Request/response schemas

### Record write request (`PUT /v1/records/{name}`, `POST /add`, `POST /remove`)

```json
{
  "ip": "198.51.100.10",
  "type": "A",
  "text": "optional for TXT",
  "target": "optional for CNAME/MX",
  "priority": 10,
  "ttl": 60,
  "zone": "example.com",
  "propagate": true
}
```

Field semantics:

- `type`: `A`, `AAAA`, `TXT`, `CNAME`, `MX`.
- If `type` is omitted, server infers it from fields (`text` -> TXT, `target+priority` -> MX, `target` -> CNAME, else IP-based A/AAAA).
- `ttl`: if `0` or omitted, defaults to `DEFAULT_TTL`.
- `zone`: if omitted, inferred by best matching known zone, then `DEFAULT_ZONE`, then parent labels of name.
- `propagate`: optional; default `true`.

Success response (`200`): normalized record object:

```json
{
  "id": 1,
  "name": "app.example.com.",
  "type": "A",
  "ip": "198.51.100.10",
  "text": "",
  "target": "",
  "priority": 0,
  "ttl": 60,
  "zone": "example.com.",
  "updated_at": "2026-02-22T12:00:00Z",
  "version": 1771800000000000000,
  "source": "node-a"
}
```

### Zone write request (`PUT /v1/zones/{zone}`)

```json
{
  "ns": ["ns1.example.net", "ns2.example.net"],
  "soa_ttl": 60,
  "propagate": true
}
```

Rules:

- If `ns` is empty, server tries existing zone NS, then `DEFAULT_NS`.
- If NS is still empty, request fails (`400`).
- `soa_ttl` defaults to `DEFAULT_TTL` when omitted/0.

Success response (`200`):

```json
{
  "zone": "example.com.",
  "ns": ["ns1.example.net.", "ns2.example.net."],
  "soa_ttl": 60,
  "serial": 1771800000,
  "updated_at": "2026-02-22T12:00:00Z"
}
```

### Sync event request (`POST /v1/sync/event`)

```json
{
  "origin_node": "node-a",
  "op": "set",
  "record": { "name": "app.example.com.", "type": "A", "ip": "198.51.100.10", "ttl": 60, "zone": "example.com." },
  "name": "",
  "type": "",
  "zone": "",
  "version": 1771800000000000000,
  "event_time": "2026-02-22T12:00:00Z",
  "zone_config": null
}
```

`op` rules:

- `set`/`add`/`remove`: require `record`.
- `delete`: requires `name`; optional `type` in (`A`,`AAAA`,`TXT`,`CNAME`,`MX`).
- `zone`: requires `zone_config`.
- If `version` is `0` or missing, server sets current timestamp-nanos.

Success response (`200`):

```json
{ "ok": true }
```

## Error model and status codes

JSON endpoints use:

```json
{ "error": "message" }
```

Status matrix:

- `200`: success
- `400`: invalid JSON, validation error, unsupported op/type, missing required route/body fields
- `401`: auth failure
- `413`: DoH payload too large
- `500`: DoH response pack failure (rare)

Common auth errors:

- control API: `{"error":"unauthorized"}`
- sync API: `{"error":"missing sync token"}` or `{"error":"invalid sync token"}`

## Query parameters

- `GET /dns-query`: `dns` (required), base64url-encoded DNS wire message.
- `DELETE /v1/records/{name}`:
  - `type` optional (`A|AAAA|TXT|CNAME|MX`)
  - `propagate` optional (`false` disables fan-out; default true)

## Idempotency and conflict behavior

- `PUT /v1/records/{name}` is logically idempotent for same desired RRset state.
- `POST /add` and `POST /remove` are member-level operations; repeated calls converge.
- `DELETE /v1/records/{name}` is idempotent from caller perspective.
- Replication conflict resolution is last-write-wins by `version`; older versions are ignored.
- Cross-node consistency is eventual (async fan-out to peers).

## Minimal integration examples

Upsert A record:

```bash
curl -sS -X PUT "http://127.0.0.1:8080/v1/records/app.example.com" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"A","ip":"198.51.100.10","ttl":60}'
```

Add second A value (RRset member):

```bash
curl -sS -X POST "http://127.0.0.1:8080/v1/records/app.example.com/add" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"A","ip":"198.51.100.11"}'
```

Delete only AAAA values for a name:

```bash
curl -sS -X DELETE "http://127.0.0.1:8080/v1/records/app.example.com?type=AAAA" \
  -H "Authorization: Bearer $API_TOKEN"
```

Upsert zone NS:

```bash
curl -sS -X PUT "http://127.0.0.1:8080/v1/zones/example.com" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ns":["ns1.example.net","ns2.example.net"],"soa_ttl":60}'
```

DoH query:

```bash
kdig @127.0.0.1 +https app.example.com A
```

## Source of truth files

- `http.go`
- `types.go`
- `util.go`
- `dns.go`
- `http_test.go`
