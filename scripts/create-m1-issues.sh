#!/bin/bash
# Create M1 Foundation issues in GitHub
# Requires: gh CLI authenticated
# Usage: ./scripts/create-m1-issues.sh

set -e

REPO="decko/flux"

echo "Creating M1 Foundation issues for $REPO..."

# Get milestone title for M1
M1_MILESTONE=$(gh api repos/$REPO/milestones --jq '.[] | select(.title == "M1: Foundation") | .title')

if [ -z "$M1_MILESTONE" ]; then
    echo "Error: M1 milestone not found. Run setup-github.sh first."
    exit 1
fi

echo "Using milestone: $M1_MILESTONE"

create_issue() {
    local title="$1"
    local labels="$2"
    local body="$3"
    
    echo "Creating: $title"
    gh issue create \
        --repo "$REPO" \
        --title "$title" \
        --label "$labels" \
        --milestone "$M1_MILESTONE" \
        --body "$body"
    sleep 1  # Rate limiting
}

# Issue 1
create_issue \
    "Initialize Go module and project structure" \
    "area/backend,type/chore,priority/high,m1/foundation,status/ready" \
    "## Context

Set up the Go module, directory structure, and basic tooling for the flux project.

## Acceptance Criteria

- [ ] Go module initialized with \`go mod init github.com/decko/flux\`
- [ ] Directory structure created per \`docs/architecture.md\`
- [ ] \`Makefile\` with build, test, lint, run targets
- [ ] \`.gitignore\` configured for Go + Node projects
- [ ] \`golangci-lint\` configuration file
- [ ] Pre-commit hooks configured (gofmt, lint, test)

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 2
create_issue \
    "Define core domain models" \
    "area/backend,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Define the core domain models that represent flux's data structures.

## Acceptance Criteria

- [ ] \`Project\` model with fields: ID, Name, RepoURL, Definition, Adapters, Pipelines
- [ ] \`Ticket\` model with fields: ID, ProjectID, ExternalID, Source, Title, Description, Status, Labels, Relationships, PRs
- [ ] \`PullRequest\` model with fields: ID, ProjectID, ExternalID, Source, Title, URL, Status, TicketIDs, Reviews
- [ ] \`PipelineRun\` model with fields: ID, ProjectID, TicketID, Orchestrator, Pipeline, Status, Phases, Cost
- [ ] All models have JSON tags
- [ ] All models have validation methods

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed (tests for validation)
- [ ] Documentation complete (godoc for all types)
- [ ] DoD review passed"

# Issue 3
create_issue \
    "Define repository interfaces" \
    "area/backend,area/database,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Define repository interfaces for database access abstraction.

## Acceptance Criteria

- [ ] \`ProjectRepository\` interface: Create, Get, List, Update, Delete
- [ ] \`TicketRepository\` interface: Create, Get, List (with filters), Update, Delete
- [ ] \`PullRequestRepository\` interface: Create, Get, List, Update, Delete
- [ ] \`PipelineRunRepository\` interface: Create, Get, List, Update
- [ ] All methods accept \`context.Context\` as first parameter
- [ ] All methods return errors (no panics)

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed (interface tests with mocks)
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 4
create_issue \
    "Implement SQLite ProjectRepository" \
    "area/backend,area/database,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Implement the SQLite repository for the Project model.

## Acceptance Criteria

- [ ] \`SQLiteProjectRepository\` struct implementing \`ProjectRepository\` interface
- [ ] Database migration for \`projects\` table
- [ ] All CRUD operations implemented
- [ ] Proper error handling and wrapping
- [ ] Connection pooling configured
- [ ] Transactions where needed

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes (with in-memory SQLite)
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed (tests written first)
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 5
create_issue \
    "Implement SQLite TicketRepository" \
    "area/backend,area/database,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Implement the SQLite repository for the Ticket model.

## Acceptance Criteria

- [ ] \`SQLiteTicketRepository\` struct implementing \`TicketRepository\` interface
- [ ] Database migration for \`tickets\` table
- [ ] All CRUD operations implemented
- [ ] List with filters (by project, status, labels)
- [ ] Relationship storage and retrieval
- [ ] Proper error handling

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 6
create_issue \
    "Implement SQLite PullRequestRepository" \
    "area/backend,area/database,type/feature,priority/medium,m1/foundation,status/ready" \
    "## Context

Implement the SQLite repository for the PullRequest model.

## Acceptance Criteria

- [ ] \`SQLitePullRequestRepository\` struct implementing \`PullRequestRepository\` interface
- [ ] Database migration for \`pull_requests\` table
- [ ] All CRUD operations implemented
- [ ] Link to tickets via \`ticket_ids\`
- [ ] Proper error handling

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 7
create_issue \
    "Implement SQLite PipelineRunRepository" \
    "area/backend,area/database,type/feature,priority/medium,m1/foundation,status/ready" \
    "## Context

Implement the SQLite repository for the PipelineRun model.

## Acceptance Criteria

- [ ] \`SQLitePipelineRunRepository\` struct implementing \`PipelineRunRepository\` interface
- [ ] Database migration for \`pipeline_runs\` table
- [ ] All CRUD operations implemented
- [ ] Phase results storage
- [ ] Cost breakdown storage
- [ ] Proper error handling

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 8
create_issue \
    "Create domain services" \
    "area/backend,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Create domain services that orchestrate business logic.

## Acceptance Criteria

- [ ] \`ProjectService\` with business logic for projects
- [ ] \`TicketService\` with business logic for tickets
- [ ] \`PullRequestService\` with business logic for PRs
- [ ] \`PipelineRunService\` with business logic for pipeline runs
- [ ] Services depend on repository interfaces (not implementations)
- [ ] Services handle validation, authorization, events

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 9
create_issue \
    "Set up Chi router and basic API structure" \
    "area/backend,area/api,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Set up the HTTP API layer using Chi router.

## Acceptance Criteria

- [ ] Chi router configured with middleware (logger, recoverer, request ID)
- [ ] Health check endpoint: \`GET /health\`
- [ ] API versioning: \`/api/v1/\` prefix
- [ ] Error handling middleware
- [ ] CORS middleware configured
- [ ] Request/response logging

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 10
create_issue \
    "Implement Project API endpoints" \
    "area/backend,area/api,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Implement REST API endpoints for projects.

## Acceptance Criteria

- [ ] \`POST /api/v1/projects\` - Create project
- [ ] \`GET /api/v1/projects\` - List projects
- [ ] \`GET /api/v1/projects/:id\` - Get project
- [ ] \`PUT /api/v1/projects/:id\` - Update project
- [ ] \`DELETE /api/v1/projects/:id\` - Delete project
- [ ] Request validation
- [ ] Proper HTTP status codes
- [ ] JSON responses

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes (integration tests)
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete (API docs)
- [ ] DoD review passed"

# Issue 11
create_issue \
    "Implement Ticket API endpoints" \
    "area/backend,area/api,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Implement REST API endpoints for tickets.

## Acceptance Criteria

- [ ] \`GET /api/v1/tickets\` - List tickets (with filters: project, status, labels)
- [ ] \`GET /api/v1/tickets/:id\` - Get ticket
- [ ] \`PUT /api/v1/tickets/:id\` - Update ticket
- [ ] Request validation
- [ ] Proper HTTP status codes
- [ ] JSON responses
- [ ] Pagination for list endpoint

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 12
create_issue \
    "Implement PullRequest API endpoints" \
    "area/backend,area/api,type/feature,priority/medium,m1/foundation,status/ready" \
    "## Context

Implement REST API endpoints for pull requests.

## Acceptance Criteria

- [ ] \`GET /api/v1/pull-requests\` - List PRs (with filters: project, status)
- [ ] \`GET /api/v1/pull-requests/:id\` - Get PR
- [ ] \`PUT /api/v1/pull-requests/:id\` - Update PR
- [ ] Request validation
- [ ] Proper HTTP status codes
- [ ] JSON responses

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 13
create_issue \
    "Implement PipelineRun API endpoints" \
    "area/backend,area/api,type/feature,priority/medium,m1/foundation,status/ready" \
    "## Context

Implement REST API endpoints for pipeline runs.

## Acceptance Criteria

- [ ] \`GET /api/v1/pipeline-runs\` - List runs (with filters: project, ticket, status)
- [ ] \`GET /api/v1/pipeline-runs/:id\` - Get run
- [ ] \`POST /api/v1/pipeline-runs\` - Trigger run
- [ ] Request validation
- [ ] Proper HTTP status codes
- [ ] JSON responses

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 14
create_issue \
    "Add configuration loading" \
    "area/backend,type/feature,priority/medium,m1/foundation,status/ready" \
    "## Context

Implement configuration loading from YAML file and environment variables.

## Acceptance Criteria

- [ ] \`Config\` struct with all configuration fields
- [ ] Load from \`flux.yaml\` file
- [ ] Override with environment variables
- [ ] Validate configuration
- [ ] Sensible defaults
- [ ] Example config file (\`flux.yaml.example\`)

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 15
create_issue \
    "Create main binary and server startup" \
    "area/backend,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Create the main binary that starts the flux server.

## Acceptance Criteria

- [ ] \`cmd/flux/main.go\` with server startup
- [ ] Load configuration
- [ ] Initialize database
- [ ] Run migrations
- [ ] Set up repositories
- [ ] Set up services
- [ ] Set up API handlers
- [ ] Graceful shutdown on SIGINT/SIGTERM
- [ ] Structured logging

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 16
create_issue \
    "Initialize frontend SPA with Vite + TypeScript" \
    "area/frontend,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Initialize the frontend SPA using Vite, TypeScript, and React.

## Acceptance Criteria

- [ ] Vite project initialized in \`web/\` directory
- [ ] TypeScript configured in strict mode
- [ ] React 18+ installed
- [ ] TanStack Router installed
- [ ] TanStack Query installed
- [ ] Tailwind CSS configured
- [ ] ESLint configured
- [ ] Prettier configured
- [ ] Basic routing structure
- [ ] API client setup

## DoD Checklist
- [ ] \`npm run typecheck\` passes
- [ ] \`npm run lint\` passes
- [ ] \`npm run test\` passes
- [ ] \`npm run build\` succeeds
- [ ] TDD followed (basic component test)
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 17
create_issue \
    "Embed frontend in Go binary" \
    "area/backend,area/frontend,type/feature,priority/high,m1/foundation,status/ready" \
    "## Context

Embed the frontend build in the Go binary for single-binary deployment.

## Acceptance Criteria

- [ ] Frontend build output in \`web/dist/\`
- [ ] Go \`embed\` package used to embed \`web/dist/\`
- [ ] Serve embedded files at root path
- [ ] Fallback to \`index.html\` for SPA routing
- [ ] Makefile target builds frontend before backend
- [ ] Development mode serves from Vite dev server

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] \`npm run build\` succeeds
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 18
create_issue \
    "Add basic authentication (JWT)" \
    "area/backend,area/api,type/feature,priority/medium,m1/foundation,status/ready" \
    "## Context

Implement basic JWT authentication for the API.

## Acceptance Criteria

- [ ] \`User\` model with ID, Email, PasswordHash, Role
- [ ] \`POST /api/v1/auth/register\` - Register user
- [ ] \`POST /api/v1/auth/login\` - Login (return JWT)
- [ ] JWT middleware for protected routes
- [ ] Password hashing with bcrypt
- [ ] Token refresh endpoint
- [ ] JWT secret from environment variable

## DoD Checklist
- [ ] \`go build ./...\` passes
- [ ] \`go test -race ./...\` passes
- [ ] \`golangci-lint run\` passes
- [ ] \`gofmt -s -d .\` shows no changes
- [ ] TDD followed
- [ ] Documentation complete
- [ ] DoD review passed"

# Issue 19
create_issue \
    "Write README with quickstart" \
    "area/docs,type/chore,priority/medium,m1/foundation,status/ready" \
    "## Context

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
- [ ] DoD review passed"

# Issue 20
create_issue \
    "Add CI/CD pipeline" \
    "type/chore,priority/medium,m1/foundation,status/ready" \
    "## Context

Set up CI/CD pipeline with GitHub Actions.

## Acceptance Criteria

- [ ] Run on push to main and PRs
- [ ] Backend: \`go build\`, \`go test -race\`, \`golangci-lint\`, \`gofmt\`
- [ ] Frontend: \`npm run typecheck\`, \`npm run lint\`, \`npm run test\`, \`npm run build\`
- [ ] Build binary artifact
- [ ] Cache Go modules and npm dependencies

## DoD Checklist
- [ ] CI pipeline passes
- [ ] Documentation complete
- [ ] DoD review passed"

echo ""
echo "All M1 issues created successfully!"
echo "Total: 20 issues"
