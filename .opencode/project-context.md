# Flux — Shared Project Context

> All agents read this file before starting any task.

## What this is

Web-based control plane for agentic software development lifecycle. Go + Chi backend, React + TypeScript frontend, SQLite database. Manages projects, tickets, PRs, and orchestrator runs (soda).

---

## Active milestone: M1 — Foundation

**Goal:** Build the core infrastructure — Go module, domain models, repositories, API, frontend SPA, auth, and embed frontend in binary.

**Status:** Harness complete, ready to start implementation.

---

## M1 Issues (in order)

| # | Issue | Status | Depends on |
|---|-------|--------|------------|
| 1 | Initialize Go module and project structure | Ready | — |
| 2 | Define core domain models | Ready | #1 |
| 3 | Define repository interfaces | Ready | #2 |
| 4 | Implement SQLite ProjectRepository | Ready | #3 |
| 5 | Implement SQLite TicketRepository | Ready | #3 |
| 6 | Implement SQLite PullRequestRepository | Ready | #3 |
| 7 | Implement SQLite PipelineRunRepository | Ready | #3 |
| 8 | Create domain services | Ready | #4, #5, #6, #7 |
| 9 | Set up Chi router and basic API structure | Ready | #1 |
| 10 | Implement Project API endpoints | Ready | #8, #9 |
| 11 | Implement Ticket API endpoints | Ready | #8, #9 |
| 12 | Implement PullRequest API endpoints | Ready | #8, #9 |
| 13 | Implement PipelineRun API endpoints | Ready | #8, #9 |
| 14 | Add configuration loading | Ready | #1 |
| 15 | Create main binary and server startup | Ready | #10-14 |
| 16 | Initialize frontend SPA with Vite + TypeScript | Ready | #1 |
| 17 | Embed frontend in Go binary | Ready | #15, #16 |
| 18 | Add basic authentication (JWT) | Ready | #10-13 |
| 19 | Write README with quickstart | Ready | #17 |
| 20 | Add CI/CD pipeline | Ready | #19 |

### Critical path

```
#1 → #2 → #3 → #4-7 (parallel) → #8 → #10-13 (parallel) → #15 → #17 → #19 → #20
     ↓                                      ↑
     #9 ────────────────────────────────────┘
     ↓
     #14 → #15
     ↓
     #16 → #17
     ↓
     #18 (parallel with #15-17)
```

### What to pick up now

**Issue #1** — Initialize Go module and project structure. No blockers.

---

## Hard constraints every agent must respect

1. **TDD first** — tests before implementation; verify failure before passing
2. **Documentation mandatory** — godoc for all public types/functions
3. **No panics in application code** — return errors, don't panic
4. **Type safety in frontend** — no `any` types in TypeScript
5. **API keys are env-only** — never in code, config files, or logs
6. **Single binary deployment** — frontend embedded in Go binary
7. **Repository pattern** — all database access through repository interfaces
8. **Adapter pattern** — all external integrations through adapter interfaces
9. **5-layer review pipeline** — Context → Domain → Security → Cross-domain → Critical → Triage
10. **Max 3 review cycles** — if still not approved, stop and ask user
11. **Worktree workflow** — all work in `.worktrees/task/<issue>-<slug>/`
12. **Branch naming** — `task/<issue-number>-<short-slug>`
13. **1 issue = 1 PR** — unless explicitly bundled
14. **Docs in same PR** — doc updates ship with code changes, never as follow-ups
15. **No new dependencies without justification** — check with go-architect first

### Anti-scope-creep rules

16. **Found a related issue while implementing?** Open a new issue, add it to the milestone, do NOT fix it in the current PR
17. **PRs > 15 changed files** — senior-qe must split the review into two passes
18. **M1 issues must not fix M2+ regressions** — use labels to queue them to the correct milestone
19. **`status/blocked` label** — always add a comment with the blocking issue number

---

## Key domain models

```
Project
├── ID, Name, RepoURL
├── Definition (language, framework, conventions)
├── Adapters (ticket sources)
└── Pipelines (orchestrators)

Ticket
├── ID, ProjectID, ExternalID, Source
├── Title, Description, Status, Labels
├── Relationships (blocks, blocked-by, relates-to)
└── PRs (linked pull requests)

PullRequest
├── ID, ProjectID, ExternalID, Source
├── Title, URL, Status
├── TicketIDs (linked tickets)
└── Reviews

PipelineRun
├── ID, ProjectID, TicketID
├── Orchestrator, Pipeline, Status
├── Phases (results per phase)
└── Cost (breakdown)
```

## Repository interfaces

```
ProjectRepository: Create, Get, List, Update, Delete
TicketRepository: Create, Get, List (with filters), Update, Delete
PullRequestRepository: Create, Get, List, Update, Delete
PipelineRunRepository: Create, Get, List, Update
```

## API structure

Base URL: `/api/v1/`

| Resource | Endpoints |
|----------|-----------|
| Projects | `POST /projects`, `GET /projects`, `GET /projects/:id`, `PUT /projects/:id`, `DELETE /projects/:id` |
| Tickets | `GET /tickets`, `GET /tickets/:id`, `PUT /tickets/:id` |
| Pull Requests | `GET /pull-requests`, `GET /pull-requests/:id`, `PUT /pull-requests/:id` |
| Pipeline Runs | `GET /pipeline-runs`, `GET /pipeline-runs/:id`, `POST /pipeline-runs` |
| Auth | `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh` |

## Agent dispatch table

| Issue | Agents to dispatch |
|-------|-------------------|
| #1 | `go-coder` (Makefile, .gitignore, golangci-lint config) |
| #2 | `go-tester` (validation tests), `go-coder` (models) |
| #3 | `go-tester` (interface tests with mocks), `go-coder` (interfaces) |
| #4-7 | `go-tester` (repository tests), `go-coder` (SQLite implementations) |
| #8 | `go-tester` (service tests), `go-coder` (services), `go-architect` (if complex) |
| #9 | `go-tester` (API tests), `go-coder` (Chi router, middleware) |
| #10-13 | `go-tester` (endpoint tests), `go-coder` (handlers) |
| #14 | `go-tester` (config tests), `go-coder` (YAML loading, env vars) |
| #15 | `go-tester` (startup tests), `go-coder` (main.go, graceful shutdown) |
| #16 | `frontend-coder` (Vite, TypeScript, TanStack setup) |
| #17 | `go-coder` (embed package), `frontend-coder` (build config) |
| #18 | `go-tester` (auth tests), `go-coder` (JWT, bcrypt), `go-architect` (auth model) |
| #19 | `technical-writer` (or `go-coder` if no writer agent) |
| #20 | `go-coder` (GitHub Actions workflow) |

## Path conventions

| Area | Path |
|------|------|
| Main binary | `cmd/flux/` |
| HTTP handlers | `internal/api/` |
| Business logic | `internal/domain/` |
| Domain types | `internal/model/` |
| Ticket adapters | `internal/adapter/ticket/` |
| Orchestrator adapters | `internal/adapter/orchestrator/` |
| Database implementations | `internal/repository/` |
| Agent worker | `internal/agent/` |
| MCP server | `internal/mcp/` |
| Configuration | `internal/config/` |
| Frontend source | `web/src/` |
| Frontend build | `web/dist/` (embedded) |
| Documentation | `docs/` |
| Migrations | `migrations/` |

## Issue and PR standards

### When creating a GitHub issue

Every agent that opens a `gh issue create` must include:

```
## Context
<Why this issue exists>

## Implementation prompts
- Dispatch agents: <list from agent dispatch table>
- Key invariants: <hard rules from this file>
- Dependent issues: <#number — blockers or conflicts>

## Acceptance criteria
- [ ] <concrete, testable criterion>
- [ ] Tests written before implementation (TDD)
- [ ] All reviewers APPROVED before PR

## Review prompts
- Reviewer must check: <specific items from 5-layer pipeline>
- Cross-domain risks: <boundary concerns>
```

### When creating a GitHub PR

Every agent that opens a `gh pr create` must include:

**`## TDD`** — mandatory:
- Which tests were written before implementation
- Test file + test name for each

**`## Review prompts`**:
- Which reviewers reviewed and their verdict
- Cross-domain concerns for human reviewer
- Known gaps intentionally not addressed

A PR with no `## TDD` section is **not mergeable**. senior-qe will mark it BLOCKED.

## Handoff contracts

```
flux-expert      ──call if issue incomplete──> feature-intake
flux-expert      ──delegate architecture──> go-architect
flux-expert      ──delegate tests──> go-tester
flux-expert      ──delegate implementation──> go-coder / frontend-coder
flux-expert      ──delegate routing──> reviewer-router
reviewer-router  ──route to──> go-reviewer, go-reviewer2, frontend-reviewer, frontend-reviewer2
flux-expert      ──delegate adversarial──> senior-qe
any agent        ──delegate exploration──> go-scout
```

## Tech stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25+, Chi router, SQLite (→ PostgreSQL) |
| Frontend | React 19, TypeScript, Vite, TanStack Router/Query, Tailwind CSS |
| Auth | JWT (Bearer tokens), bcrypt |
| Testing (BE) | Go testing package, table-driven tests |
| Testing (FE) | Vitest, Testing Library |
| Package manager | `go mod` (backend), `npm` (frontend) |
| Infrastructure | Single binary, GitHub Actions CI |

## Development commands

### Backend

```bash
go build ./...
go test -race -cover ./...
golangci-lint run
gofmt -s -w .
```

### Frontend

```bash
cd web
npm install
npm run typecheck
npm run lint
npm run test
npm run build
```

### Full build

```bash
make build  # frontend + backend
make run    # run the binary
make dev    # hot reload (if configured)
```
