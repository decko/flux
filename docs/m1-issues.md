# M1: Foundation Issues

Create these issues in GitHub under the "M1: Foundation" milestone.

## Issue 1: Initialize Go module and project structure

**Labels:** `area/backend`, `type/chore`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Set up the Go module, directory structure, and basic tooling for the flux project.

## Acceptance Criteria

- [ ] Go module initialized with `go mod init github.com/decko/flux`
- [ ] Directory structure created per `docs/architecture.md`
- [ ] `Makefile` with build, test, lint, run targets
- [ ] `.gitignore` configured for Go + Node projects
- [ ] `golangci-lint` configuration file
- [ ] Pre-commit hooks configured (gofmt, lint, test)

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed (tests for Makefile targets if applicable)
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 2: Define core domain models

**Labels:** `area/backend`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Define the core domain models that represent flux's data structures.

## Acceptance Criteria

- [ ] `Project` model with fields: ID, Name, RepoURL, Definition, Adapters, Pipelines
- [ ] `Ticket` model with fields: ID, ProjectID, ExternalID, Source, Title, Description, Status, Labels, Relationships, PRs
- [ ] `PullRequest` model with fields: ID, ProjectID, ExternalID, Source, Title, URL, Status, TicketIDs, Reviews
- [ ] `PipelineRun` model with fields: ID, ProjectID, TicketID, Orchestrator, Pipeline, Status, Phases, Cost
- [ ] All models have JSON tags
- [ ] All models have validation methods

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed (tests for validation)
- [ ] Documentation complete (godoc for all types)
- [ ] DoD review passed
```

---

## Issue 3: Define repository interfaces

**Labels:** `area/backend`, `area/database`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Define repository interfaces for database access abstraction.

## Acceptance Criteria

- [ ] `ProjectRepository` interface: Create, Get, List, Update, Delete
- [ ] `TicketRepository` interface: Create, Get, List (with filters), Update, Delete
- [ ] `PullRequestRepository` interface: Create, Get, List, Update, Delete
- [ ] `PipelineRunRepository` interface: Create, Get, List, Update
- [ ] All methods accept `context.Context` as first parameter
- [ ] All methods return errors (no panics)

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed (interface tests with mocks)
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 4: Implement SQLite repository for Project

**Labels:** `area/backend`, `area/database`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement the SQLite repository for the Project model.

## Acceptance Criteria

- [ ] `SQLiteProjectRepository` struct implementing `ProjectRepository` interface
- [ ] Database migration for `projects` table
- [ ] All CRUD operations implemented
- [ ] Proper error handling and wrapping
- [ ] Connection pooling configured
- [ ] Transactions where needed

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes (with in-memory SQLite)
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed (tests written first)
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 5: Implement SQLite repository for Ticket

**Labels:** `area/backend`, `area/database`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement the SQLite repository for the Ticket model.

## Acceptance Criteria

- [ ] `SQLiteTicketRepository` struct implementing `TicketRepository` interface
- [ ] Database migration for `tickets` table
- [ ] All CRUD operations implemented
- [ ] List with filters (by project, status, labels)
- [ ] Relationship storage and retrieval
- [ ] Proper error handling

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 6: Implement SQLite repository for PullRequest

**Labels:** `area/backend`, `area/database`, `type/feature`, `priority/medium`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement the SQLite repository for the PullRequest model.

## Acceptance Criteria

- [ ] `SQLitePullRequestRepository` struct implementing `PullRequestRepository` interface
- [ ] Database migration for `pull_requests` table
- [ ] All CRUD operations implemented
- [ ] Link to tickets via `ticket_ids`
- [ ] Proper error handling

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 7: Implement SQLite repository for PipelineRun

**Labels:** `area/backend`, `area/database`, `type/feature`, `priority/medium`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement the SQLite repository for the PipelineRun model.

## Acceptance Criteria

- [ ] `SQLitePipelineRunRepository` struct implementing `PipelineRunRepository` interface
- [ ] Database migration for `pipeline_runs` table
- [ ] All CRUD operations implemented
- [ ] Phase results storage
- [ ] Cost breakdown storage
- [ ] Proper error handling

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 8: Create domain services

**Labels:** `area/backend`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Create domain services that orchestrate business logic.

## Acceptance Criteria

- [ ] `ProjectService` with business logic for projects
- [ ] `TicketService` with business logic for tickets
- [ ] `PullRequestService` with business logic for PRs
- [ ] `PipelineRunService` with business logic for pipeline runs
- [ ] Services depend on repository interfaces (not implementations)
- [ ] Services handle validation, authorization, events

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 9: Set up Chi router and basic API structure

**Labels:** `area/backend`, `area/api`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Set up the HTTP API layer using Chi router.

## Acceptance Criteria

- [ ] Chi router configured with middleware (logger, recoverer, request ID)
- [ ] Health check endpoint: `GET /health`
- [ ] API versioning: `/api/v1/` prefix
- [ ] Error handling middleware
- [ ] CORS middleware configured
- [ ] Request/response logging

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 10: Implement Project API endpoints

**Labels:** `area/backend`, `area/api`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement REST API endpoints for projects.

## Acceptance Criteria

- [ ] `POST /api/v1/projects` - Create project
- [ ] `GET /api/v1/projects` - List projects
- [ ] `GET /api/v1/projects/:id` - Get project
- [ ] `PUT /api/v1/projects/:id` - Update project
- [ ] `DELETE /api/v1/projects/:id` - Delete project
- [ ] Request validation
- [ ] Proper HTTP status codes
- [ ] JSON responses

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes (integration tests)
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete (API docs)
- [ ] DoD review passed
```

---

## Issue 11: Implement Ticket API endpoints

**Labels:** `area/backend`, `area/api`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement REST API endpoints for tickets.

## Acceptance Criteria

- [ ] `GET /api/v1/tickets` - List tickets (with filters: project, status, labels)
- [ ] `GET /api/v1/tickets/:id` - Get ticket
- [ ] `PUT /api/v1/tickets/:id` - Update ticket
- [ ] Request validation
- [ ] Proper HTTP status codes
- [ ] JSON responses
- [ ] Pagination for list endpoint

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 12: Implement PullRequest API endpoints

**Labels:** `area/backend`, `area/api`, `type/feature`, `priority/medium`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement REST API endpoints for pull requests.

## Acceptance Criteria

- [ ] `GET /api/v1/pull-requests` - List PRs (with filters: project, status)
- [ ] `GET /api/v1/pull-requests/:id` - Get PR
- [ ] `PUT /api/v1/pull-requests/:id` - Update PR
- [ ] Request validation
- [ ] Proper HTTP status codes
- [ ] JSON responses

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 13: Implement PipelineRun API endpoints

**Labels:** `area/backend`, `area/api`, `type/feature`, `priority/medium`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement REST API endpoints for pipeline runs.

## Acceptance Criteria

- [ ] `GET /api/v1/pipeline-runs` - List runs (with filters: project, ticket, status)
- [ ] `GET /api/v1/pipeline-runs/:id` - Get run
- [ ] `POST /api/v1/pipeline-runs` - Trigger run
- [ ] Request validation
- [ ] Proper HTTP status codes
- [ ] JSON responses

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 14: Add configuration loading

**Labels:** `area/backend`, `type/feature`, `priority/medium`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement configuration loading from YAML file and environment variables.

## Acceptance Criteria

- [ ] `Config` struct with all configuration fields
- [ ] Load from `flux.yaml` file
- [ ] Override with environment variables
- [ ] Validate configuration
- [ ] Sensible defaults
- [ ] Example config file (`flux.yaml.example`)

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 15: Create main binary and server startup

**Labels:** `area/backend`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Create the main binary that starts the flux server.

## Acceptance Criteria

- [ ] `cmd/flux/main.go` with server startup
- [ ] Load configuration
- [ ] Initialize database
- [ ] Run migrations
- [ ] Set up repositories
- [ ] Set up services
- [ ] Set up API handlers
- [ ] Graceful shutdown on SIGINT/SIGTERM
- [ ] Structured logging

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 16: Initialize frontend SPA with Vite + TypeScript

**Labels:** `area/frontend`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Initialize the frontend SPA using Vite, TypeScript, and React.

## Acceptance Criteria

- [ ] Vite project initialized in `web/` directory
- [ ] TypeScript configured in strict mode
- [ ] React 18+ installed
- [ ] TanStack Router installed
- [ ] TanStack Query installed
- [ ] Tailwind CSS configured (or alternative)
- [ ] ESLint configured
- [ ] Prettier configured
- [ ] Basic routing structure
- [ ] API client setup

## DoD Checklist

- [ ] `npm run typecheck` passes
- [ ] `npm run lint` passes
- [ ] `npm run test` passes
- [ ] `npm run build` succeeds
- [ ] TDD followed (basic component test)
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 17: Embed frontend in Go binary

**Labels:** `area/backend`, `area/frontend`, `type/feature`, `priority/high`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Embed the frontend build in the Go binary for single-binary deployment.

## Acceptance Criteria

- [ ] Frontend build output in `web/dist/`
- [ ] Go `embed` package used to embed `web/dist/`
- [ ] Serve embedded files at root path
- [ ] Fallback to `index.html` for SPA routing
- [ ] Makefile target builds frontend before backend
- [ ] Development mode serves from Vite dev server

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] `npm run build` succeeds
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 18: Add basic authentication (JWT)

**Labels:** `area/backend`, `area/api`, `type/feature`, `priority/medium`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Implement basic JWT authentication for the API.

## Acceptance Criteria

- [ ] `User` model with ID, Email, PasswordHash, Role
- [ ] `POST /api/v1/auth/register` - Register user
- [ ] `POST /api/v1/auth/login` - Login (return JWT)
- [ ] JWT middleware for protected routes
- [ ] Password hashing with bcrypt
- [ ] Token refresh endpoint
- [ ] JWT secret from environment variable

## DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 19: Write README with quickstart

**Labels:** `area/docs`, `type/chore`, `priority/medium`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Write a README with project overview and quickstart guide.

## Acceptance Criteria

- [ ] Project description
- [ ] Features list
- [ ] Installation instructions
- [ ] Configuration guide
- [ ] Quickstart (run flux locally)
- [ ] Development setup
- [ ] Contributing link
- [ ] License

## DoD Checklist

- [ ] Documentation complete
- [ ] DoD review passed
```

---

## Issue 20: Add CI/CD pipeline

**Labels:** `type/chore`, `priority/medium`, `m1/foundation`, `status/ready`

**Body:**
```markdown
## Context

Set up CI/CD pipeline with GitHub Actions.

## Acceptance Criteria

- [ ] Run on push to main and PRs
- [ ] Backend: `go build`, `go test -race`, `golangci-lint`, `gofmt`
- [ ] Frontend: `npm run typecheck`, `npm run lint`, `npm run test`, `npm run build`
- [ ] Build binary artifact
- [ ] Cache Go modules and npm dependencies

## DoD Checklist

- [ ] CI pipeline passes
- [ ] Documentation complete
- [ ] DoD review passed
```
