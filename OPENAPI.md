# OpenAPI Specification (dns-server)

This document provides an OpenAPI 3.1 description for integration work.

Notes:

- It reflects current behavior in `http.go`.
- Some runtime defaults (like open auth when tokens are empty) are deployment-config dependent and described in endpoint notes.

```yaml
openapi: 3.1.0
info:
  title: dns-server HTTP API
  version: 1.0.0
  description: |
    Authoritative DNS control API with DoH and peer sync.

    Auth behavior is environment-dependent:
    - If API_TOKEN is set, /v1/records* and /v1/zones* require bearer token or X-API-Token.
    - If SYNC_TOKEN is set, /v1/sync/event requires X-Sync-Token.
servers:
  - url: http://localhost:8080

tags:
  - name: Health
  - name: DoH
  - name: Records
  - name: Zones
  - name: Sync

paths:
  /healthz:
    get:
      tags: [Health]
      summary: Health check
      responses:
        '200':
          description: Node is healthy
          content:
            application/json:
              schema:
                type: object
                required: [ok, node_id, uptime_sec]
                properties:
                  ok:
                    type: boolean
                  node_id:
                    type: string
                  uptime_sec:
                    type: integer

  /dns-query:
    get:
      tags: [DoH]
      summary: DNS over HTTPS GET
      description: DNS wire message is passed in `dns` query parameter encoded as base64url (raw, no padding).
      parameters:
        - in: query
          name: dns
          required: true
          schema:
            type: string
      responses:
        '200':
          description: DNS wire response
          content:
            application/dns-message:
              schema:
                type: string
                format: binary
        '400':
          description: Invalid request (missing/invalid dns parameter or invalid wire message)
          content:
            text/plain:
              schema:
                type: string
        '413':
          description: DNS message too large
          content:
            text/plain:
              schema:
                type: string
        '500':
          description: Failed to encode DNS response
          content:
            text/plain:
              schema:
                type: string
    post:
      tags: [DoH]
      summary: DNS over HTTPS POST
      requestBody:
        required: true
        content:
          application/dns-message:
            schema:
              type: string
              format: binary
      responses:
        '200':
          description: DNS wire response
          content:
            application/dns-message:
              schema:
                type: string
                format: binary
        '400':
          description: Empty/invalid payload
          content:
            text/plain:
              schema:
                type: string
        '413':
          description: DNS message too large
          content:
            text/plain:
              schema:
                type: string
        '500':
          description: Failed to encode DNS response
          content:
            text/plain:
              schema:
                type: string

  /v1/records:
    get:
      tags: [Records]
      summary: List records
      security:
        - bearerAuth: []
        - apiTokenHeader: []
      responses:
        '200':
          description: Records list
          content:
            application/json:
              schema:
                type: object
                required: [records]
                properties:
                  records:
                    type: array
                    items:
                      $ref: '#/components/schemas/ARecord'
        '401':
          $ref: '#/components/responses/Unauthorized'

  /v1/records/{name}:
    parameters:
      - $ref: '#/components/parameters/RecordName'
    put:
      tags: [Records]
      summary: Upsert record (set mode)
      description: Replaces RRset members for the same `(name, type)` when accepted.
      security:
        - bearerAuth: []
        - apiTokenHeader: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UpsertRecordRequest'
      responses:
        '200':
          description: Upserted record
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ARecord'
        '400':
          $ref: '#/components/responses/ErrorJSON'
        '401':
          $ref: '#/components/responses/Unauthorized'
    delete:
      tags: [Records]
      summary: Delete records by name (optionally by type)
      security:
        - bearerAuth: []
        - apiTokenHeader: []
      parameters:
        - in: query
          name: type
          required: false
          schema:
            $ref: '#/components/schemas/RecordType'
          description: If set, delete only this type.
        - in: query
          name: propagate
          required: false
          schema:
            type: string
            enum: ['false']
          description: Set to `false` to disable peer propagation.
      responses:
        '200':
          description: Delete accepted
          content:
            application/json:
              schema:
                type: object
                required: [deleted, type, version]
                properties:
                  deleted:
                    type: string
                  type:
                    type: string
                  version:
                    type: integer
                    format: int64
        '400':
          $ref: '#/components/responses/ErrorJSON'
        '401':
          $ref: '#/components/responses/Unauthorized'

  /v1/records/{name}/add:
    post:
      tags: [Records]
      summary: Add one RR member
      security:
        - bearerAuth: []
        - apiTokenHeader: []
      parameters:
        - $ref: '#/components/parameters/RecordName'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UpsertRecordRequest'
      responses:
        '200':
          description: Added/updated member
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ARecord'
        '400':
          $ref: '#/components/responses/ErrorJSON'
        '401':
          $ref: '#/components/responses/Unauthorized'

  /v1/records/{name}/remove:
    post:
      tags: [Records]
      summary: Remove one RR member
      security:
        - bearerAuth: []
        - apiTokenHeader: []
      parameters:
        - $ref: '#/components/parameters/RecordName'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UpsertRecordRequest'
      responses:
        '200':
          description: Remove accepted
          content:
            application/json:
              schema:
                type: object
                required: [removed, type, version]
                properties:
                  removed:
                    type: string
                  type:
                    type: string
                  version:
                    type: integer
                    format: int64
        '400':
          $ref: '#/components/responses/ErrorJSON'
        '401':
          $ref: '#/components/responses/Unauthorized'

  /v1/zones:
    get:
      tags: [Zones]
      summary: List zones
      security:
        - bearerAuth: []
        - apiTokenHeader: []
      responses:
        '200':
          description: Zones list
          content:
            application/json:
              schema:
                type: object
                required: [zones]
                properties:
                  zones:
                    type: array
                    items:
                      $ref: '#/components/schemas/ZoneConfig'
        '401':
          $ref: '#/components/responses/Unauthorized'

  /v1/zones/{zone}:
    put:
      tags: [Zones]
      summary: Upsert zone
      security:
        - bearerAuth: []
        - apiTokenHeader: []
      parameters:
        - in: path
          name: zone
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UpsertZoneRequest'
      responses:
        '200':
          description: Upserted zone
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ZoneConfig'
        '400':
          $ref: '#/components/responses/ErrorJSON'
        '401':
          $ref: '#/components/responses/Unauthorized'

  /v1/sync/event:
    post:
      tags: [Sync]
      summary: Apply replication event
      description: |
        Internal endpoint for peer replication.
        Requires X-Sync-Token when SYNC_TOKEN is configured.
      security:
        - syncTokenHeader: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/SyncEvent'
      responses:
        '200':
          description: Event accepted
          content:
            application/json:
              schema:
                type: object
                required: [ok]
                properties:
                  ok:
                    type: boolean
        '400':
          $ref: '#/components/responses/ErrorJSON'
        '401':
          description: Missing or invalid sync token
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
    apiTokenHeader:
      type: apiKey
      in: header
      name: X-API-Token
    syncTokenHeader:
      type: apiKey
      in: header
      name: X-Sync-Token

  parameters:
    RecordName:
      in: path
      name: name
      required: true
      schema:
        type: string
      description: DNS name. Server normalizes to lowercase FQDN.

  responses:
    Unauthorized:
      description: Unauthorized
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
          examples:
            unauthorized:
              value:
                error: unauthorized
    ErrorJSON:
      description: Request validation or decoding error
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'

  schemas:
    ErrorResponse:
      type: object
      required: [error]
      properties:
        error:
          type: string

    RecordType:
      type: string
      enum: [A, AAAA, TXT, CNAME, MX]

    ARecord:
      type: object
      required:
        - name
        - type
        - ip
        - ttl
        - zone
        - updated_at
        - version
        - source
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
          description: Lowercase FQDN with trailing dot.
        type:
          $ref: '#/components/schemas/RecordType'
        ip:
          type: string
          description: IPv4/IPv6 for A/AAAA; empty for TXT/CNAME/MX.
        text:
          type: string
        target:
          type: string
        priority:
          type: integer
          minimum: 0
          maximum: 65535
        ttl:
          type: integer
          minimum: 1
        zone:
          type: string
          description: Lowercase FQDN with trailing dot.
        updated_at:
          type: string
          format: date-time
        version:
          type: integer
          format: int64
        source:
          type: string

    ZoneConfig:
      type: object
      required: [zone, ns, soa_ttl, serial, updated_at]
      properties:
        zone:
          type: string
        ns:
          type: array
          items:
            type: string
        soa_ttl:
          type: integer
          minimum: 1
        serial:
          type: integer
          minimum: 0
        updated_at:
          type: string
          format: date-time

    UpsertRecordRequest:
      type: object
      additionalProperties: false
      properties:
        ip:
          type: string
        type:
          $ref: '#/components/schemas/RecordType'
        text:
          type: string
        target:
          type: string
        priority:
          type: integer
          minimum: 0
          maximum: 65535
        ttl:
          type: integer
          minimum: 0
        zone:
          type: string
        propagate:
          type: boolean
      description: |
        Unknown fields are rejected.
        If `type` is omitted, server infers it from body fields.

    UpsertZoneRequest:
      type: object
      additionalProperties: false
      properties:
        ns:
          type: array
          items:
            type: string
        soa_ttl:
          type: integer
          minimum: 0
        propagate:
          type: boolean
      description: Unknown fields are rejected.

    SyncEvent:
      type: object
      additionalProperties: false
      required: [op]
      properties:
        origin_node:
          type: string
        op:
          type: string
          enum: [set, add, remove, delete, zone]
        record:
          $ref: '#/components/schemas/SyncRecordInput'
        name:
          type: string
        type:
          $ref: '#/components/schemas/RecordType'
        zone:
          type: string
        version:
          type: integer
          format: int64
        event_time:
          type: string
          format: date-time
        zone_config:
          $ref: '#/components/schemas/ZoneConfig'
      description: |
        Conditional requirements by `op`:
        - set/add/remove: `record` required
        - delete: `name` required; `type` optional
        - zone: `zone_config` required

    SyncRecordInput:
      type: object
      additionalProperties: false
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
        type:
          $ref: '#/components/schemas/RecordType'
        ip:
          type: string
        text:
          type: string
        target:
          type: string
        priority:
          type: integer
          minimum: 0
          maximum: 65535
        ttl:
          type: integer
          minimum: 0
        zone:
          type: string
        updated_at:
          type: string
          format: date-time
        version:
          type: integer
          format: int64
        source:
          type: string
```

## Integration notes

- JSON body endpoints reject unknown fields.
- Names are normalized to FQDN with trailing dot in stored/returned values.
- `propagate` defaults to true when omitted.
- `DELETE /v1/records/{name}?propagate=false` disables fan-out.
- Sync conflict policy is version-based (older versions are ignored).
