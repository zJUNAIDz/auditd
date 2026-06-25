<div align="center">

# 🛡️ auditd

### Tamper-evident, multi-tenant audit logging as a service

*An append-only audit trail backed by a cryptographic hash chain — so you can **prove** your logs were never altered.*

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev) [![Gin](https://img.shields.io/badge/Gin-HTTP-008ECF?logo=gin&logoColor=white)](https://gin-gonic.com) [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-18-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org) [![HMAC](https://img.shields.io/badge/HMAC-SHA--256-success)](https://en.wikipedia.org/wiki/HMAC) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](#-license)

</div>

---

## ✨ What is this?

`auditd` is a self-hostable HTTP service for recording **audit events** — *"who did what, to which resource, and when."* Unlike a plain log table, every event is linked to the one before it through an **HMAC-SHA256 hash chain**. If anyone tampers with a single row (even with direct database access), the chain breaks and a verification call will pinpoint exactly where.

It is built for **multi-tenancy** from the ground up: each tenant gets its own API key and its own secret, and every tenant's events form an independent, isolated chain.

### Why hash-chained logs?

A regular audit table answers *"what happened?"*. A hash-chained audit table also answers *"can I trust that this is the complete, unmodified record?"* — which is exactly the question auditors, compliance frameworks (SOC 2, HIPAA, PCI-DSS), and incident responders care about.

---

## 🌟 Features

| Feature | Description |
| --- | --- |
| 🔗 **Tamper-evident chain** | Each event's hash is `HMAC(event ‖ prev_hash)`. Altering any event invalidates every event after it. |
| 🏢 **Multi-tenant** | Per-tenant API keys, per-tenant HMAC secrets, fully isolated chains. |
| ⚡ **Async ingestion** | Writes are accepted instantly into an in-memory queue and persisted by a worker pool — low-latency `POST`s. |
| 🔁 **Reliable workers** | Retry with exponential backoff and graceful drain-on-shutdown so queued events aren't lost. |
| 🔒 **Append-only storage** | Postgres `RULE`s reject every `UPDATE`/`DELETE` at the database level. |
| 🧮 **Concurrency-safe** | `pg_advisory_xact_lock` per tenant serializes chain writes without global contention. |
| 🔎 **Rich filtering** | Query events by actor, action, resource, and time range with pagination. |
| ✅ **Chain verification** | A single endpoint re-computes the entire chain and reports the first broken link. |
| 📤 **Streaming export** | Export events as **CSV** or **JSONL/NDJSON**, streamed row-by-row (constant memory). |
| 🚀 **Graceful lifecycle** | Auto-migrations on boot, in-flight request draining, and clean worker shutdown. |
| 🗃️ **Type-safe SQL** | Queries generated with [`sqlc`](https://sqlc.dev) — no hand-written scanning. |

---

## 🏗️ Architecture

### High-level overview

```mermaid
flowchart LR
    Client([Client / SDK])

    subgraph API["auditd service"]
        direction TB
        MW["Auth Middleware<br/>(tenant / admin)"]
        H["Gin Handlers"]
        Q["In-memory Queue<br/>(buffered channel)"]
        WP["Worker Pool<br/>(N workers)"]
        SVC["Audit Service<br/>(hashing + chain)"]
    end

    DB[(PostgreSQL<br/>append-only)]

    Client -->|"Bearer API key"| MW --> H
    H -->|"POST /events"| Q
    Q --> WP --> SVC
    H -->|"reads / verify / export"| SVC
    SVC <--> DB
```

### Write path: how an event becomes a link in the chain

The ingest path is intentionally split: the HTTP request returns the moment an event is **accepted** into the queue, while persistence (and the cryptographic chaining) happens asynchronously in a worker.

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant Q as Queue
    participant W as Worker
    participant DB as PostgreSQL

    C->>H: POST /v1/events  (Bearer API key)
    H->>H: Resolve tenant (cached 5 min)
    H->>Q: Enqueue payload
    alt queue full
        H-->>C: 503 Service Unavailable
    else accepted
        H-->>C: 200 { id, status: "accepted" }
    end

    W->>DB: BEGIN tx
    W->>DB: pg_advisory_xact_lock(tenant)
    W->>DB: SELECT last event hash
    W->>W: hash = HMAC(event ‖ prev_hash)
    W->>DB: INSERT event (prev_hash, hash)
    W->>DB: COMMIT
    Note over W: retry w/ backoff on failure (max 5)
```

### The hash chain

Every event stores the hash of the previous event, forming a linked, verifiable chain per tenant. The first event links to a fixed `GENESIS_HASH`.

```mermaid
flowchart LR
    G["GENESIS_HASH"] --> E1
    subgraph E1["Event 1"]
        H1["hash₁ = HMAC(e₁ ‖ GENESIS)"]
    end
    subgraph E2["Event 2"]
        H2["hash₂ = HMAC(e₂ ‖ hash₁)"]
    end
    subgraph E3["Event 3"]
        H3["hash₃ = HMAC(e₃ ‖ hash₂)"]
    end
    E1 -->|prev_hash| E2 -->|prev_hash| E3

    style G fill:#2d3748,color:#fff
```

> [!IMPORTANT]
> The hash is computed as an `HMAC-SHA256` over `id | tenant_id | actor_id | action | resource_id | timestamp | prev_hash`, keyed with the **tenant's secret**. Because the chain is keyed and linked, an attacker with raw DB access cannot recompute valid hashes without the secret, and cannot silently delete or reorder rows.

### Component layout

```mermaid
flowchart TB
    main["cmd/server/main.go<br/><i>wiring & lifecycle</i>"]

    subgraph internal
        config["config<br/><i>env loading</i>"]
        handler["handler<br/><i>HTTP endpoints</i>"]
        middleware["middleware<br/><i>auth (tenant/admin)</i>"]
        queue["queue<br/><i>buffered channel</i>"]
        worker["worker<br/><i>pool + retry</i>"]
        service["service<br/><i>hashing, chain, tenants</i>"]
        model["model<br/><i>request/payload types</i>"]
        db["db<br/><i>sqlc-generated</i>"]
    end

    PG[(PostgreSQL)]

    main --> config & handler & middleware & queue & worker & service
    handler --> service & queue
    worker --> service
    service --> db --> PG
    middleware --> service
```

---

## 🗄️ Data model

```mermaid
erDiagram
    TENANTS ||--o{ AUDIT_EVENTS : owns

    TENANTS {
        uuid id PK
        text name
        text api_key UK
        text hmac_secret
        bool active
        timestamptz created_at
    }

    AUDIT_EVENTS {
        uuid id PK
        uuid tenant_id FK
        text actor_id
        text actor_type "user|service|system"
        text action
        text resource_type
        text resource_id
        jsonb metadata
        timestamptz timestamp
        text prev_hash
        text hash
        timestamptz created_at
    }
```

`audit_events` is protected by `RULE no_update`/`RULE no_delete` so the table is **append-only at the database level**, and indexed on `(tenant_id, timestamp)`, `(tenant_id, action)`, `actor_id`, and `resource_id` for fast filtering.

---

## 🚀 Getting started

### Prerequisites

- **Go** 1.26+
- **Docker** (for local Postgres) or an existing PostgreSQL 18 instance
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI *(optional — migrations also run automatically on startup)*
- [`sqlc`](https://sqlc.dev) *(optional — only needed to regenerate query code)*

### 1. Clone & configure

```bash
git clone https://github.com/zjunaidz/auditd.git
cd auditd
```

Create a `.env` file in the project root:

```dotenv
DB_URL=postgres://postgres:type_xi@localhost:5432/auditd?sslmode=disable
HMAC_SECRET=change-me-to-a-long-random-string
ADMIN_KEY=change-me-to-a-strong-admin-key
PORT=8080
```

| Variable | Required | Description |
| --- | :---: | --- |
| `DB_URL` | ✅ | PostgreSQL connection string |
| `HMAC_SECRET` | ✅ | Fallback/server HMAC secret used when a tenant secret is absent |
| `ADMIN_KEY` | ✅ | Bearer key that protects the `/admin` tenant-management routes |
| `PORT` | — | HTTP port (defaults to `8080`) |

### 2. Start PostgreSQL

```bash
make compose-up      # spins up postgres:18 via compose.dev.yaml
```

### 3. Run the service

```bash
go run ./cmd/server
```

On boot the service connects to Postgres, **runs migrations automatically**, starts the worker pool, and serves HTTP.

```
Successfully connected to the database
Database migration completed
Server is running on port 8080
```

> [!TIP]
> Migrations also run automatically at startup, but you can apply or roll them back manually:
> ```bash
> make migrate-up      # apply all migrations
> make migrate-down    # roll back the last migration
> make sqlc            # regenerate type-safe query code
> ```

---

## 📡 API reference

All `/v1` routes require an `Authorization: Bearer <api_key>` header (the tenant API key).
The `/admin` route requires `Authorization: Bearer <ADMIN_KEY>`.

### 🩺 Health

```http
GET /health
```
Returns `200 {"status":"ok"}` when the database is reachable.

### 🏢 Create a tenant (admin)

```bash
curl -X POST http://localhost:8080/admin/tenants \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"acme-corp"}'
```

```json
{
  "id": "8f3b...",
  "tenant": { "id": "8f3b...", "name": "acme-corp", "active": true },
  "api_key": "p7Qf...generated..."
}
```

> [!WARNING]
> The `api_key` is shown **once** at creation. Store it securely — it's the credential the tenant uses to ingest and read events.

### 📥 Record an event

```bash
curl -X POST http://localhost:8080/v1/events \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "actor_id": "user_42",
    "actor_type": "user",
    "action": "document.delete",
    "resource_type": "document",
    "resource_id": "doc_99",
    "metadata": { "ip": "203.0.113.7", "reason": "cleanup" }
  }'
```

```json
{ "id": "c1a2...", "status": "accepted" }
```

`actor_type` must be one of `user`, `service`, or `system`.

### 📃 List & filter events

```bash
curl "http://localhost:8080/v1/events?action=document.delete&limit=20&offset=0" \
  -H "Authorization: Bearer $API_KEY"
```

| Query param | Description |
| --- | --- |
| `actor_id` | Filter by actor |
| `action` | Filter by action |
| `resource_id` / `resource_type` | Filter by resource |
| `start_time` / `end_time` | RFC3339 time-range bounds |
| `limit` / `offset` | Pagination (default `limit=10`) |

### ✅ Verify the chain

```bash
curl http://localhost:8080/v1/verify \
  -H "Authorization: Bearer $API_KEY"
```

```json
{ "verified": true, "event_count": 1284 }
```

If a row has been tampered with, the response is `409 Conflict` and identifies the first broken event:

```json
{ "verified": false, "event_count": 1284, "broken_at": "c1a2-...-bad" }
```

### 📤 Export events

Stream a date-bounded export as CSV (default) or JSONL:

```bash
# CSV
curl "http://localhost:8080/v1/export?from=2026-01-01T00:00:00Z&to=2026-12-31T23:59:59Z" \
  -H "Authorization: Bearer $API_KEY" -o audit-export.csv

# JSONL / NDJSON
curl "http://localhost:8080/v1/export?format=jsonl&from=...&to=..." \
  -H "Authorization: Bearer $API_KEY" -o audit-export.ndjson
```

Both `from` and `to` are required and must be RFC3339 timestamps. Rows are streamed and flushed individually, so exports run in constant memory regardless of result size.

---

## 🧠 Design notes

- **Why a queue + workers?** Ingestion latency stays low and independent of write contention. The HTTP handler only enqueues; persistence and hashing happen off the request path. A full queue returns `503` rather than blocking.
- **Why per-tenant advisory locks?** The hash chain requires reading the *previous* hash before inserting. `pg_advisory_xact_lock(tenant)` serializes writes **per tenant** within a transaction — correctness without a global lock.
- **Why a tenant cache?** `ResolveTenant` caches API-key → tenant lookups for 5 minutes to avoid a DB round-trip on every request.
- **Graceful shutdown.** On `SIGINT`/`SIGTERM` the HTTP server stops accepting connections, drains in-flight requests, then the workers drain the remaining queue before exit.

---

## 🗂️ Project structure

```
auditd/
├── cmd/server/          # entrypoint: wiring, migrations, lifecycle
├── db/
│   ├── migrations/      # golang-migrate up/down SQL
│   └── queries/         # sqlc source queries
├── internal/
│   ├── config/          # env-based configuration
│   ├── db/              # sqlc-generated, type-safe DB layer
│   ├── handler/         # Gin HTTP handlers (events, admin, export)
│   ├── middleware/      # tenant & admin auth
│   ├── model/           # request/payload types
│   ├── queue/           # buffered in-memory event queue
│   ├── service/         # hashing, chain logic, tenant mgmt
│   └── worker/          # worker pool with retry/backoff
├── compose.dev.yaml     # local Postgres
├── Makefile             # compose / migrate / sqlc helpers
└── sqlc.yaml            # sqlc codegen config
```

---

## 🧰 Tech stack

- **Language:** Go 1.26
- **HTTP:** [Gin](https://gin-gonic.com)
- **Database:** PostgreSQL 18 via [pgx/v5](https://github.com/jackc/pgx)
- **Migrations:** [golang-migrate](https://github.com/golang-migrate/migrate)
- **Codegen:** [sqlc](https://sqlc.dev)
- **Crypto:** `crypto/hmac` + `crypto/sha256` (stdlib)

---

## 📄 License

Released under the [MIT License](LICENSE).

<div align="right">
<sub>This README was generated with AI</sub>
</div>
