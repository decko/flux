# Flux

Web-based control plane for agentic software development lifecycle.

## What is Flux?

Flux provides visibility and orchestration for teams using AI agents to develop software. It wraps tools like [soda](https://github.com/decko/soda) with a dashboard for:

- **Projects**: Manage multiple repositories
- **Tickets**: View and track issues from GitHub, Jira, Linear
- **Relationships**: See how tickets relate to each other
- **Pull Requests**: Link PRs to their tickets
- **Pipeline Runs**: Trigger and monitor agent runs
- **Cost Tracking**: Understand agent costs per ticket/project

## Architecture

```
┌─────────────┐     JSON-RPC      ┌──────────────────┐
│  Flux Core  │◄─────────────────►│  Agent Workers   │
│  (Go/Chi)   │                   │  (soda, custom)  │
│             │     MCP Server    │                  │
│  - Projects │◄─────────────────►│  (write-back)    │
│  - Tickets  │                   └──────────────────┘
│  - PRs      │
│  - Pipelines│     Adapters
│  - Users    │◄─────────────────► GitHub / Jira / ...
└─────────────┘
     │
     ▼ (embed SPA)
┌─────────────┐
│  Frontend   │
│  (Vite+TS)  │
└─────────────┘
```

See [docs/architecture.md](docs/architecture.md) for full details.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/decko/flux.git
cd flux

# Build
make build

# Run
make run

# Open http://localhost:8080
```

## Configuration

Create `flux.yaml` (see `flux.yaml.example`):

```yaml
server:
  host: 0.0.0.0
  port: 8080

database:
  driver: sqlite
  dsn: ./flux.db

auth:
  jwt_secret: ${FLUX_JWT_SECRET}

projects:
  - name: my-project
    repo: https://github.com/user/repo
    adapters:
      - type: github
        token: ${GITHUB_TOKEN}
    orchestrators:
      - type: soda
        path: ./soda.yaml
```

## Development

```bash
# Backend only
make backend
make test
make lint

# Frontend only
cd web
npm install
npm run dev

# Full build
make build
```

## Roadmap

- **M1: Foundation** - Go + Chi + SQLite + basic API
- **M2: GitHub Adapter** - Issues + PRs sync from GitHub
- **M3: soda Integration** - Trigger and monitor soda runs
- **M4: Frontend** - SPA with TanStack Router/Query
- **M5: Self-Host** - Flux manages flux

See [docs/roadmap.md](docs/roadmap.md) for details.

## Contributing

See [AGENTS.md](AGENTS.md) for AI-assisted development guidelines.

## License

Apache-2.0 - see [LICENSE](LICENSE)
