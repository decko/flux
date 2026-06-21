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
| **M2: GitHub Adapter** | 🔜 Next | Issues + PRs sync from GitHub |
| **M3: soda Integration** | 📋 Planned | Trigger and monitor soda pipeline runs |
| **M4: Frontend** | 📋 Planned | Full SPA dashboard with real data |
| **M5: Self-Host** | 📋 Planned | Flux manages flux |

## Contributing

See [AGENTS.md](AGENTS.md) for AI-assisted development guidelines and workflow conventions.

## License

Apache-2.0 — see [LICENSE](LICENSE)
