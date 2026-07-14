# Flux — Shared Project Context

> All agents read this file before starting any task.

## What this is

Web-based control plane for agentic software development lifecycle. Go + Chi backend, React + TypeScript frontend, SQLite database. Manages projects, tickets, PRs, and orchestrator runs (soda).

---

## Active phase: System Review Remediation

**Status:** M1-M12 complete. System review #1 (2026-07-06) produced 17 findings. 8 resolved, 11 remaining. Working through findings in severity order.

**Last updated:** 2026-07-14

---

## Current Work: System Review Findings

### Resolved (8/17)
#293 (SQLite FK), #289 (sync trigger), #290 (status machine), #291 (idempotency), #292 (status bypass), #295 (env leak), #296 (arg injection), #281 (stranded-state)

### Open (11/17 — sorted by priority)

| # | Sev | Issue |
|---|-----|-------|
| **277** | HIGH | Project create/update not admin-gated — any user can configure outbound access |
| 278 | MED | dev-secret fallback + NO_AUTH bypass enables silent JWT forgery |
| 279 | MED | Fire-and-forget webhook ops use request context — goroutines killed mid-flight |
| 280 | MED | Webhook registration has no atomicity — partial failure = silent data loss |
| 282 | MED | WebhookCreator defined but never wired — auto-registration is dead code |
| 283 | MED | GitHub installations/repos endpoints exposed to all authenticated users |
| 284 | LOW | Per-project sync status in-memory only — lost on restart |
| 285 | LOW | README claims Jira/Linear adapter support — neither exists |
| 286 | LOW | Architecture doc diagrams include unimplemented components |
| 287 | LOW | go.mod/go.sum out of sync |
| 288 | LOW | Tracked build artifacts in web/dist/ |

### Other open issues

| # | Area | Title |
|---|------|-------|
| 268 | backend | Add author login and avatar URL to Ticket and PR models |
| 269 | backend | Add server-side sorting to Ticket and PR API |
| 270 | frontend | Display author avatar and name on Ticket/PR cards |
| 271 | frontend | Add sort dropdown to Ticket/PR pages |
| 272 | frontend | Show contributor faces on PR and Ticket cards |
| 239 | test | M12 functional tests |

---

### What to pick up now

**Issue #277** — Project create/update not admin-gated. Single-route-group fix, low-lines, high-impact. No blockers.

---

## Completed Milestones

| Milestone | Issues | Key Features |
|-----------|--------|-------------|
| M1 Foundation | #1-20, #42 | Go + Chi + SQLite + React SPA + JWT auth |
| M2 GitHub Adapter | #44-51, #60 | Ticket/PR sync, relationships, auto-sync |
| M3 soda | #62-66 | Orchestrator adapter, pipeline trigger |
| M4 Frontend | #72-77, #84-86 | Dashboard, pages, auth flow |
| M5 Self-Host | #91-93 | Docs, config, background workers |
| M6 Audit | #113-117 | Audit events, role middleware, API |
| M7 Audit UI | #118-120 | Frontend viewer, hash chain, retention |
| DB Infra | #134-136 | golang-migrate, pure-Go SQLite, sqlx |
| GitHub App | #142-145 | AppAuth, installation IDs, per-project sync |
| M8 Discovery | #153-160 | Installation/repo browser, project creation wizard |
| M9 Triggers | #171-177 | TriggerService, dedup, configurable rules, CLI admin |
| M10 UI | #183-190 | DB-backed trigger rules, CRUD API, project detail with rule editor |
| M11 Webhooks | #205-228 | Webhook receiver (HMAC), auto-registration, lifecycle, gitleaks |
| M12 Hardening | #231-249 | sync.enabled flag, webhook health, audit ingress, admin sync gate, rotation |

---

## Key Domain Models

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

## Repository Interfaces

```
ProjectRepository: Create, Get, List, Update, Delete
TicketRepository: Create, Get, List (with filters), Update, Delete
PullRequestRepository: Create, Get, List, Update, Delete
PipelineRunRepository: Create, Get, List, Update
```

## API Structure

Base URL: `/api/v1/`

| Resource | Endpoints |
|----------|-----------|
| Projects | POST /projects, GET /projects, GET /projects/:id, PUT /projects/:id, DELETE /projects/:id |
| Tickets | GET /tickets, GET /tickets/:id, PUT /tickets/:id |
| Pull Requests | GET /pull-requests, GET /pull-requests/:id, PUT /pull-requests/:id |
| Pipeline Runs | GET /pipeline-runs, GET /pipeline-runs/:id, POST /pipeline-runs |
| Auth | POST /auth/register, POST /auth/login, POST /auth/refresh |
| Admin | GET /admin/users, POST /admin/users, PUT /admin/users/:id/role, PUT /admin/users/:id/password, DELETE /admin/users/:id |
| Audit | GET /audit-events, GET /audit/integrity |
| Webhooks | POST /webhooks/github |
| Sync | GET /sync/status, POST /sync/trigger, POST /projects/:id/sync |
| Triggers | GET /projects/:id/trigger-rules, POST/PUT/DELETE trigger rules |

---

## Hard Constraints Every Agent Must Respect

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

## Path Conventions

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

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25+, Chi router, SQLite (→ PostgreSQL) |
| Frontend | React 19, TypeScript, Vite, TanStack Router/Query, Tailwind CSS |
| Auth | JWT (Bearer tokens), bcrypt |
| Testing (BE) | Go testing package, table-driven tests |
| Testing (FE) | Vitest, Testing Library |
| Package manager | `go mod` (backend), `bun` (frontend) |
| Infrastructure | Single binary, GitHub Actions CI |

## Development Commands

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
bun install
bun run typecheck
bun run lint
bun run test
bun run build
```

### Full build

```bash
make build  # frontend + backend
make run    # run the binary
make dev    # hot reload
```

## Handoff Contracts

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
