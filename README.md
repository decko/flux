# Flux

Web-based control plane for agentic software development lifecycle.

## What is Flux?

Flux provides visibility and orchestration for teams using AI agents to develop software. It manages projects, tickets, pull requests, and pipeline runs through a single binary that embeds a React SPA.

**Features (M1):**

- **Projects** — Create and manage software projects with adapters and pipelines
- **Tickets** — View and filter tickets with pagination (GitHub, Jira, Linear)
- **Pull Requests** — Track PRs linked to tickets
- **Pipeline Runs** — Trigger and monitor agentic pipeline executions
- **REST API** — Full CRUD with JWT authentication, pagination, and filtering
- **SPA Dashboard** — React frontend embedded in a single Go binary
- **Configuration** — YAML config with environment variable overrides
- **Graceful Shutdown** — SIGINT/SIGTERM handling with connection draining

## Quickstart (Self-Host)

Flux manages its own development. Here's how to set it up.

### 1. Build

```bash
git clone https://github.com/decko/flux
cd flux
make build  # or: go build -o flux ./cmd/flux/
```

### 2. Configure

```bash
# Required: JWT secret for auth (at least 16 characters)
export JWT_SECRET=your-secret-key-at-least-16-chars

# Required for GitHub sync: create a classic token at https://github.com/settings/tokens
export GITHUB_TOKEN=ghp_your-token-here

# Copy and edit the example config
cp flux.yaml.example flux.yaml
# Edit owner/repo in flux.yaml if needed
```

### 3. Run

```bash
./flux
# Open http://localhost:8080 — you'll be redirected to /login
# Register an account, then all pages are available
```

### 4. Sync from GitHub

```bash
# Login via API to get a token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"your@email.com","password":"your-password"}' | jq -r '.token')

# Trigger a sync
curl -X POST http://localhost:8080/api/v1/sync/trigger \
  -H "Authorization: Bearer $TOKEN"
```

## Quick Start

```bash
# Clone and build (frontend builds automatically)
git clone https://github.com/decko/flux.git
cd flux
make build

# Set JWT secret (required, min 16 chars)
export JWT_SECRET="your-secret-key-at-least-16-chars"

# Run with defaults (in-memory SQLite, port 8080)
./bin/flux

# Open http://localhost:8080
```

## Configuration

Create `flux.yaml` (see `flux.yaml.example`):

```yaml
server:
  port: 8080

database:
  path: flux.db   # or ":memory:" for ephemeral

cors:
  origin: "*"     # restrict in production

logging:
  level: info     # debug, info, warn, error
```

Override with environment variables:

```bash
export FLUX_SERVER_PORT=3000
export FLUX_DATABASE_PATH=/data/flux.db
export FLUX_CORS_ORIGIN=https://app.example.com
export FLUX_LOGGING_LEVEL=debug
export JWT_SECRET=your-secret-key-here
```

Precedence: file defaults → YAML file → environment variables.

## API Reference

Base URL: `/api/v1`

### Authentication

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/register` | No | Register user (`email`, `password`) |
| POST | `/auth/login` | No | Login → `{"token": "..."}` |
| POST | `/auth/refresh` | Bearer | Refresh token → `{"token": "..."}` |

All other endpoints require `Authorization: Bearer <token>`.

### Projects

| Method | Path | Description |
|--------|------|-------------|
| POST | `/projects` | Create project (returns 201 + Location) |
| GET | `/projects` | List all projects |
| GET | `/projects/{id}` | Get project by ID |
| PUT | `/projects/{id}` | Update project |
| DELETE | `/projects/{id}` | Delete project |

### Tickets

| Method | Path | Description |
|--------|------|-------------|
| GET | `/tickets` | List (filter: `?project_id=&status=&labels=`) |
| GET | `/tickets?page=1&limit=20` | Paginated list (defaults: page=1, limit=20, max 100) |
| GET | `/tickets/{id}` | Get ticket by ID |
| PUT | `/tickets/{id}` | Update ticket |

### Pull Requests

| Method | Path | Description |
|--------|------|-------------|
| GET | `/pull-requests` | List (filter: `?project_id=&status=`) |
| GET | `/pull-requests/{id}` | Get PR by ID |
| PUT | `/pull-requests/{id}` | Update PR |

### Pipeline Runs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/pipeline-runs` | List (filter: `?project_id=&ticket_id=&status=`) |
| GET | `/pipeline-runs/{id}` | Get run by ID |
| POST | `/pipeline-runs` | Trigger new run (returns 201 + Location) |

### Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check → `ok` |

## Development

```bash
# Backend
go build ./...
go test -race -cover ./...
golangci-lint run

# Frontend
cd web
npm install
npm run dev        # Vite dev server
npm run typecheck
npm run lint
npm run test

# Full build (frontend → backend)
make build
```

## Architecture

```
                    ┌──────────────────┐
                    │   Agent Workers  │
                    │  (soda, custom)  │
                    └────────┬─────────┘
                             │ (future: MCP/JSON-RPC)
┌─────────────┐              │
│  Go Binary   │    ┌────────▼─────────┐
│              │    │  Adapters        │
│  ┌────────┐  │    │  (GitHub, Jira)  │
│  │ Chi API │◄─┼────┤  (future)       │
│  ├────────┤  │    └──────────────────┘
│  │Domain  │  │
│  ├────────┤  │    ┌──────────────────┐
│  │ SQLite │  │    │  Embedded SPA    │
│  └────────┘  │    │  (Vite + React   │
│       │      │    │   + TanStack)    │
│       ▼      │    └──────────────────┘
│  //go:embed  │
│  web/dist/   │
└─────────────┘
```

Tech stack: Go 1.25+, Chi v5, SQLite, React 19, TypeScript, Vite, TanStack Router, TanStack Query, Tailwind CSS.

## Roadmap

| Milestone | Status | Description |
|-----------|--------|-------------|
| **M1: Foundation** | ✅ Complete | Go + Chi + SQLite + API + Auth + SPA embed |
| **M2: GitHub Adapter** | ✅ Complete | Issues + PRs sync from GitHub, relationships, auto-sync |
| **M3: soda Integration** | ✅ Complete | Orchestrator adapter, pipeline trigger |
| **M4: Frontend** | ✅ Complete | Full SPA dashboard with TanStack Router + Query |
| **M5: Self-Host** | ✅ Complete | Background workers, config, docs |
| **M6: Audit** | ✅ Complete | Hash-chained audit events, role middleware, API |
| **M7: Audit UI** | ✅ Complete | Audit viewer, hash chain, retention policy |
| **M8: Discovery** | ✅ Complete | GitHub installation/repo browser, project wizard |
| **M9: Triggers** | ✅ Complete | TriggerService, dedup, configurable rules, CLI admin |
| **M10: Trigger UI** | ✅ Complete | DB-backed trigger rules, CRUD API, rule editor |
| **M11: Webhooks** | ✅ Complete | HMAC webhook receiver, auto-registration, lifecycle |
| **M12: Sync & Webhook Hardening** | 🔜 In Progress | Deterministic IDs, sync.enabled, webhook health, audit ingress, secret rotation |

## Contributing

See [AGENTS.md](AGENTS.md) for AI-assisted development guidelines and workflow conventions.

## License

Apache-2.0 — see [LICENSE](LICENSE)
