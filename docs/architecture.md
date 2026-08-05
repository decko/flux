# Flux Architecture

## Overview

Flux is a web-based control plane for agentic software development lifecycle. It provides visibility and orchestration for teams using AI agents to develop software, wrapping tools like [soda](https://github.com/decko/soda) with a dashboard for projects, tickets, relationships, agent runs, and PRs.

## Goals

- **Self-hosting**: Flux manages its own development via soda/agents
- **Multi-project**: One instance manages multiple repos
- **Pluggable**: Adapters for ticket sources, orchestrators, and agents
- **Team-ready**: Multi-user with roles

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Flux Core                             │
│                     (Go / Chi / SQLite)                       │
├─────────────────────────────────────────────────────────────┤
│  API Layer (REST)                                            │
│  ├── Projects    ├── Tickets    ├── Pipeline Runs           │
│  ├── PRs         ├── Users      ├── Sync/Webhooks/Audit     │
├─────────────────────────────────────────────────────────────┤
│  Domain Layer                                                │
│  ├── Project Service     ├── Ticket Service                 │
│  ├── Pipeline Service    ├── Sync Service                   │
│  ├── Trigger Service     ├── Audit Service                  │
├─────────────────────────────────────────────────────────────┤
│  Adapter Layer                                               │
│  ├── Ticket Adapters (GitHub)                               │
│  ├── SCM Adapters (GitHub)                                  │
│  └── Orchestrator Adapters (soda)                           │
├─────────────────────────────────────────────────────────────┤
│  Infrastructure                                              │
│  └── Repository (SQLite → PostgreSQL)                        │
└─────────────────────────────────────────────────────────────┘
         │                                    │
         ▼                                    ▼
┌─────────────────┐              ┌─────────────────────────┐
│   Frontend SPA  │              │    External Services    │
│  (Vite + TS +   │              │  GitHub │ soda          │
│   TanStack)     │              └─────────────────────────┘
└─────────────────┘
```

> **Planned (not yet implemented):** WebSocket live updates, Event Bus
> (in-process → NATS/Redis), MCP Server (agent write-back), Agent Workers
> (JSON-RPC, local/remote), and additional adapters (Jira, Linear, GitLab,
> custom orchestrators). These appear in the Future Considerations section;
> do not design integrations against them until implemented.

## Core Concepts

### Project

A software project with a repository, ticket source, and pipeline configuration.

```go
type Project struct {
    ID          string
    Name        string
    RepoURL     string
    Definition  ProjectDefinition  // language, conventions, architecture
    Adapters    []AdapterConfig    // ticket sources
    Pipelines   []PipelineConfig   // orchestrator configs
}
```

### Ticket

A unit of work from an external source (GitHub Issue, Jira ticket, etc.).

```go
type Ticket struct {
    ID           string
    ProjectID    string
    ExternalID   string           // e.g., GitHub issue number
    Source       TicketSource     // github, jira, linear
    Title        string
    Description  string
    Status       TicketStatus
    Labels       []string
    Relationships []Relationship  // blocks, blocked-by, relates-to
    PRs          []string         // linked PR IDs
}
```

### Relationship

Links between tickets (blocks, blocked-by, relates-to, parent-child).

```go
type Relationship struct {
    Type     RelationType  // blocks, blocked-by, relates-to, parent, child
    TargetID string
}
```

### Pipeline Run

An execution of an orchestrator (e.g., soda) on a ticket.

```go
type PipelineRun struct {
    ID           string
    ProjectID    string
    TicketID     string
    Orchestrator string           // soda, custom
    Pipeline     string           // default, quick-fix, custom name
    Status       RunStatus        // pending, running, completed, failed
    Phases       []PhaseResult
    StartedAt    time.Time
    CompletedAt  *time.Time
    Cost         *CostBreakdown
}
```

### Pull Request

A code change linked to one or more tickets.

```go
type PullRequest struct {
    ID          string
    ProjectID   string
    ExternalID  string
    Source      PRSource          // github, gitlab
    Title       string
    Status      PRStatus          // open, merged, closed
    TicketIDs   []string
    Reviews     []Review
}
```

## Adapter Interfaces

### Ticket Adapter

Reads/writes tickets from external sources.

```go
type TicketAdapter interface {
    Name() string
    ListTickets(ctx context.Context, projectID string) ([]Ticket, error)
    GetTicket(ctx context.Context, projectID, externalID string) (*Ticket, error)
    CreateTicket(ctx context.Context, ticket *Ticket) (*Ticket, error)
    UpdateTicket(ctx context.Context, ticket *Ticket) error
    SyncRelationships(ctx context.Context, projectID string) error
    Health(ctx context.Context) error
}
```

### SCM Adapter

Reads pull requests and reviews from source code management systems.

```go
type SCMAdapter interface {
    Name() string
    ListPullRequests(ctx context.Context, projectID string) ([]PullRequest, error)
    GetPullRequest(ctx context.Context, projectID, externalID string) (*PullRequest, error)
    ListReviews(ctx context.Context, projectID, externalID string) ([]Review, error)
    Health(ctx context.Context) error
}
```

### Orchestrator adapter

Triggers and monitors pipeline tools.

```go
type OrchestratorAdapter interface {
    Name() string
    Trigger(ctx context.Context, run *PipelineRun) error
    Status(ctx context.Context, runID string) (*PipelineRun, error)
    Cancel(ctx context.Context, runID string) error
    Logs(ctx context.Context, runID string) (<-chan LogEntry, error)
}
```

### Agent worker (planned)

Executes agent tasks (planner, coder, reviewer, QE). **Not yet implemented** — no agent worker code exists; the interface below is the intended contract.

```go
type AgentWorker interface {
    ID() string
    Capabilities() []AgentType  // planner, coder, reviewer, qe
    Execute(ctx context.Context, task AgentTask) (*AgentResult, error)
    Health(ctx context.Context) error
}
```

### Repository

Storage abstraction. All methods use value types (not pointers). Each repository has a corresponding filter type for List operations. Get/Update/Delete return `ErrNotFound` on missing entities.

```go
type ProjectRepository interface {
    Create(ctx context.Context, project Project) error
    Get(ctx context.Context, id string) (Project, error)
    List(ctx context.Context, filter ProjectFilter) ([]Project, error)
    Update(ctx context.Context, project Project) error
    Delete(ctx context.Context, id string) error
}

type TicketRepository interface {
    Create(ctx context.Context, ticket Ticket) error
    Get(ctx context.Context, id string) (Ticket, error)
    List(ctx context.Context, filter TicketFilter) ([]Ticket, error)
    Update(ctx context.Context, ticket Ticket) error
    Delete(ctx context.Context, id string) error
}

type PullRequestRepository interface {
    Create(ctx context.Context, pr PullRequest) error
    Get(ctx context.Context, id string) (PullRequest, error)
    List(ctx context.Context, filter PullRequestFilter) ([]PullRequest, error)
    Update(ctx context.Context, pr PullRequest) error
    Delete(ctx context.Context, id string) error
}

type PipelineRunRepository interface {
    Create(ctx context.Context, run PipelineRun) error
    Get(ctx context.Context, id string) (PipelineRun, error)
    List(ctx context.Context, filter PipelineRunFilter) ([]PipelineRun, error)
    Update(ctx context.Context, run PipelineRun) error
    // No Delete — pipeline runs are immutable records
}

type SyncStatusRepository interface {
    Upsert(ctx context.Context, status SyncStatusRow) error
    GetByProjectID(ctx context.Context, projectID string) (SyncStatusRow, error)
    List(ctx context.Context) ([]SyncStatusRow, error)
}
```

Per-project sync status is persisted in the `sync_status` table: one row per
project, keyed by `project_id` (primary key with `ON DELETE CASCADE` from
`projects`). `SyncService` writes through `SyncStatusRepository` after every
sync pass and loads the rows on startup, so per-project status (and the
aggregate dashboard fields derived from it) survive process restarts.

## Data Flow

### Ticket sync

```
[GitHub] --webhook/poll--> [TicketAdapter] ──┐
                                              ├──> [SyncService] --> [Repository] --> [DB]
[GitHub] --webhook/poll--> [SCMAdapter] ─────┘
```

SyncService periodically polls both adapters and upserts tickets and pull
requests into the local repository. It supports both scheduled (Run) and
one-shot (SyncNow) synchronization with error isolation between adapters.

### Pipeline execution

```
[Frontend] --trigger--> [API] --> [PipelineService] --> [OrchestratorAdapter] --> [soda]
                                     |                                                   ^
                                     v                                                   |
                               [Repository] <------ status polling -----------------------
```

### Agent write-back (MCP) — planned

**Not yet implemented.** Intended flow when the MCP server ships:

```
[Agent] --MCP--> [MCP Server] --> [Flux API] --> [Repository]
```

## MVP Scope (Self-Host Milestone)

Flux manages its own development.

### Must have

1. **Project management**: Add flux repo as a project
2. **GitHub adapter**: Sync issues and PRs from flux repo
3. **Relationship detection**: Auto-detect from issue references (#123) and labels
4. **soda orchestrator adapter**: Trigger soda runs, track status
5. **Basic UI**:
   - Project list/detail
   - Ticket list/detail with relationships graph
   - PR list linked to tickets
   - Trigger soda on a ticket
   - View pipeline run status
6. **Auth**: Single user (expand later)
7. **SQLite storage**

### Nice to have

1. WebSocket live updates
2. Ticket relationship graph visualization
3. Cost tracking per ticket/project
4. Agent worker registration (JSON-RPC)

### Out of scope for MVP

- Multi-user/roles
- Jira/Linear adapters
- Custom agent workers
- MCP server for write-back
- Project onboarding agent

## Tech Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Backend | Go + Chi | Lightweight, stdlib-compatible |
| Database | SQLite (→ PostgreSQL) | Simple start, adapter for migration |
| API | REST | Standard, real-time updates via polling (WebSocket planned) |
| Frontend | Vite + TypeScript + TanStack Router/Query | SPA, embeddable in Go binary |
| Orchestrator | soda (pluggable) | Existing tool, JSON-RPC interface |
| Auth | JWT | Stateless, simple |

## Directory Structure

```
flux/
├── cmd/
│   └── flux/              # Main binary
├── internal/
│   ├── api/               # HTTP handlers, routes, middleware
│   ├── domain/            # Business logic services
│   ├── model/             # Domain types (Project, Ticket, etc.)
│   ├── adapter/
│   │   ├── ticket/        # GitHub adapter (Jira/Linear planned)
│   │   ├── scm/           # SCM adapters (GitHub)
│   │   └── orchestrator/  # soda adapter
│   ├── repository/        # SQLite implementations
│   ├── agent/             # Agent worker client/server (planned)
│   ├── mcp/               # MCP server for write-back (planned)
│   └── config/            # Configuration loading
├── pkg/                   # Public packages (if needed)
├── web/                   # Frontend SPA source
│   ├── src/
│   └── dist/              # Built assets (embedded)
├── docs/                  # Documentation
├── migrations/            # Database migrations
├── Makefile
├── go.mod
└── README.md
```

## Configuration

```yaml
# flux.yaml
server:
  host: 0.0.0.0
  port: 8080

database:
  driver: sqlite
  dsn: ./flux.db

auth:
  jwt_secret: ${FLUX_JWT_SECRET}

projects:
  - name: flux
    repo: https://github.com/decko/flux
    adapters:
      - type: github
        token: ${GITHUB_TOKEN}
    orchestrators:
      - type: soda
        path: ./soda.yaml

sync:
  interval: 5m
```

## Security Considerations

- Tokens/secrets via environment variables, never in config files
- JWT for API auth, short-lived tokens
- CORS configured for SPA origin
- Rate limiting on API endpoints
- Input validation on all endpoints

## Future Considerations

Planned components (not yet implemented — see System Architecture note):

- **WebSocket live updates**: Push ticket/PR/run changes to the frontend
- **Event Bus**: In-process pub/sub, later NATS/Redis for multi-node
- **MCP Server**: Agent write-back to the flux API
- **Agent Workers**: JSON-RPC client/server for planner, coder, reviewer, QE
- **Additional adapters**: Jira, Linear (tickets); GitLab (SCM); custom orchestrators
- **Event sourcing**: For audit trail and replay
- **Plugin system**: For custom adapters without recompiling
- **Metrics**: Prometheus endpoints
- **Multi-tenancy**: Isolated projects per team
- **Agent marketplace**: Registry of agent workers
