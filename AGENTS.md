# AGENTS.md — Project Guide for AI Coding Agents

## Build, Test, Lint

```bash
# Backend (Go)
go build ./...
go test -race -cover ./...
golangci-lint run
gofmt -s -w .

# Frontend (TypeScript/React)
cd web
npm install
npm run typecheck
npm run lint
npm run test
npm run build
```

## Pre-commit Hooks

Install git hooks manually (for now):

```bash
cp .githooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Hooks run on `git commit`:
- `gofmt -l .` - rejects unformatted Go code
- `golangci-lint run` - rejects lint errors
- `go test -race ./...` - rejects failing tests
- `npm run typecheck` (in web/) - rejects type errors
- `npm run lint` (in web/) - rejects lint errors

## Architecture

**flux** is a web-based control plane for agentic software development lifecycle.

### Backend (Go)

Key packages:
- `cmd/flux/` - Main binary, server startup
- `internal/api/` - HTTP handlers, routes, middleware (Chi)
- `internal/domain/` - Business logic services
- `internal/model/` - Domain types (Project, Ticket, PR, PipelineRun)
- `internal/adapter/ticket/` - Ticket source adapters (GitHub, Jira, Linear)
- `internal/adapter/orchestrator/` - Orchestrator adapters (soda)
- `internal/repository/` - Database implementations (SQLite → PostgreSQL)
- `internal/agent/` - Agent worker client/server
- `internal/mcp/` - MCP server for agent write-back
- `internal/config/` - Configuration loading

### Frontend (TypeScript/React)

- `web/src/` - React SPA source
- TanStack Router for routing
- TanStack Query for data fetching
- Embedded in Go binary via `embed` package

### Database

- Start with SQLite (simple, embedded)
- Repository pattern for abstraction
- Migrate to PostgreSQL later via adapter

## Critical Invariants

1. **TDD is MANDATORY**. Write failing test → verify it fails → implement → verify it passes. Never skip.
2. **Documentation is MANDATORY**. All public types/functions have godoc comments. Update architecture docs.
3. **No panics in application code**. Return errors, don't panic.
4. **Type safety in frontend**. No `any` types in TypeScript.
5. **API keys are env-only**. Never in code, config files, or logs.
6. **Single binary deployment**. Frontend embedded in Go binary.
7. **Repository pattern**. All database access through repository interfaces.
8. **Adapter pattern**. All external integrations through adapter interfaces.

## Git Workflow (MANDATORY)

### ⛔ NEVER commit directly to `main`

All work happens through the worktree workflow below.

### ✅ Always work in a git worktree under `.worktrees/`

```bash
# 1. Create a worktree for the task
git worktree add -b task/<slug> .worktrees/task/<slug> main

# 2. Work inside the worktree
cd .worktrees/task/<slug>

# 3. Commit changes in the worktree
git add .
git commit -m "feat: description"
```

The `.worktrees/` directory must be in `.gitignore`.

### ✅ Branch naming convention

```
task/<github-issue-number>-<short-slug>
```

Examples: `task/42-github-adapter`, `task/7-sqlite-repository`

### ✅ The worktree is temporary

After PR merge:
```bash
git worktree remove .worktrees/task/<slug>
git branch -D task/<slug>
```

## Definition of Done (DoD) — Reviewer Gate

**Two-step review before every PR:**

1. **Agent verifies checklist** - implementing agent runs all checks
2. **Reviewer agent verifies** - delegate to go-reviewer for mechanical verification
3. **PR created** - after reviewer approval

Do NOT proceed to PR creation until reviewer agent signs off.

### DoD Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes (all tests green)
- [ ] `golangci-lint run` passes
- [ ] `gofmt -s -d .` shows no changes
- [ ] `npm run typecheck` passes (in web/)
- [ ] `npm run lint` passes (in web/)
- [ ] `npm run test` passes (in web/)
- [ ] No new dependencies without justification
- [ ] No public API signatures changed without approval
- [ ] All new code has tests (TDD - tests written first)
- [ ] All public types/functions have godoc comments
- [ ] Architecture doc updated if needed
- [ ] No security issues (secrets, unsafe patterns)

### Reviewer Handoff Format

```markdown
## PR Ready for DoD Review

**Branch:** task/<issue>-<slug>
**Summary:** [1-2 lines]

DoD checklist: [all items checked by agent, ready for reviewer verification]
```

**Reviewer responds with:**
- ✅ **Approved** - proceed to PR creation
- ❌ **Changes needed** - list of failing items

## Agent Swarm

The `flux-expert` agent orchestrates development. It delegates to:

| Agent | Model | Role |
|-------|-------|------|
| flux-expert | v4-pro | Orchestrator (default agent) |
| feature-intake | qwen3.7-max | Impact assessment before planning |
| go-architect | kimi-k2.6 | Architecture decisions |
| go-tester | v4-flash | Write tests (TDD) |
| go-coder | v4-flash (max) | Go implementation |
| frontend-coder | v4-flash (max) | TypeScript/React implementation |
| reviewer-router | v4-pro (high) | Route to reviewers |
| go-reviewer | qwen3.7-max | Go review (5-layer pipeline) |
| go-reviewer2 | kimi-k2.7 | Go review (5-layer pipeline, architecture focus) |
| frontend-reviewer | qwen3.7-max | Frontend review (5-layer pipeline) |
| frontend-reviewer2 | kimi-k2.7 | Frontend review (5-layer pipeline, UX focus) |
| senior-qe | v4-pro (high) | Adversarial final gate (cross-domain, requirement fit) |
| go-scout | v4-flash | Codebase exploration |

### Review Loop

```
tests → implement → route → review → fix → review → fix → review → senior-qe → PR
                                          ↑ cycle 1    ↑ cycle 2    ↑ cycle 3 → STOP
```

Max 3 review cycles. If still not approved, stops and asks for user guidance.
After reviewers pass, senior-qe runs an adversarial cross-domain probe. If senior-qe returns NEEDS CHANGES, the loop re-enters at step 8.

## Ticket Assignment

**Before starting any work, assign the GitHub issue to the project owner (`decko`).**

```bash
gh issue edit <issue-number> --add-assignee "decko"
```

If no GitHub issue exists, create one first:

```bash
gh issue create \
  --title "<title>" \
  --body "## Context\n\n## Acceptance Criteria\n\n## DoD Checklist\n- [ ] go build\n- [ ] go test -race\n- [ ] golangci-lint\n- [ ] gofmt\n- [ ] npm typecheck\n- [ ] npm lint\n- [ ] npm test\n- [ ] TDD followed\n- [ ] Documentation complete\n- [ ] DoD review passed"
```

Then assign it before writing any code.

## Complete Workflow Summary

```
1. Create or identify the GitHub issue → assign to decko
2. Create a git worktree under .worktrees/task/<slug>
3. Plan the approach (use go-architect for complex decisions)
4. Write tests first (delegate to go-tester)
5. Verify tests fail (RED)
6. Implement (delegate to go-coder)
7. Verify tests pass (GREEN)
8. Run all DoD checks
9. Request review (delegate to go-reviewer)
10. Fix any review findings
11. Create PR after reviewer approval
12. After merge → clean up worktree and branch
```

## Resuming After Interruption

If the agent session crashes or restarts, check for in-progress work.

### Resume Protocol

When a new session initializes:

**1. Check for open issues assigned to decko:**
```bash
gh issue list --assignee decko --state open
```

**2. For each open issue, check if a worktree branch exists:**
```bash
git branch -a | grep "task/<issue-number>"
```

**3. If a worktree exists → active task:**
- Enter the worktree: `cd .worktrees/task/<issue-number>-*`
- `git status` → see uncommitted changes
- `git log --oneline -5` → see what's been committed
- Re-read the issue body
- Continue implementation

**4. If no worktree exists → issue is queued:**
- Pick the lowest-numbered open issue in the current milestone
- Create worktree, begin work

**5. After completing a task, close the issue.**

### Why This Works

Each PR/issue is sized at ~100-200 lines. If the agent crashes, at most 200 lines are lost. The issue description contains the full scope. The worktree branch has whatever was committed. This is a stateless resume.

## Memory

Every session starts with zero conversation history. The MEMORY.md file bridges context between sessions.

### Location

```
~/.local/share/opencode/projects/flux/memory.md
```

Lives outside the repo. Not tracked by git.

### When to Read

**At session start, before anything else** - including before the resume protocol.

### When to Write

After any non-trivial decision or discovery:
- Choosing between implementation options
- Discovering a build gotcha
- Changing a workflow rule
- Completing a milestone

Format:
```markdown
### YYYY-MM-DD — Brief title
- **What:** [one sentence]
- **Decision:** [what we chose]
- **Why:** [1-2 sentences]
```

### What It Contains

| Section | Purpose |
|---------|---------|
| Active State | Which milestone, which issue, worktree path |
| Key Decisions | Dated entries with rationale |
| Conventions | Rules and patterns |
| Gotchas | Things that tripped us up |
| Completed | Closed issue numbers with description |

**This is NOT a replacement for issues.** Issues are the task tracker. MEMORY.md is the context bridge.

## Code Style

### Go

- Go 1.25+
- Standard library patterns where possible
- `chi` for HTTP routing
- `sqlite3` for database (via repository pattern)
- Error handling: return errors, wrap with context
- Logging: structured logging (consider `slog` or `zerolog`)
- Testing: table-driven tests, no external assertion libraries unless already in use

### TypeScript/React

- TypeScript strict mode
- Functional components with hooks
- TanStack Router for routing
- TanStack Query for data fetching
- Tailwind CSS for styling (or project's choice)
- Component composition over inheritance

## Testing Strategy

### TDD Workflow (MANDATORY)

1. **RED**: Write a failing test
2. **VERIFY**: Run the test - it MUST fail
3. **GREEN**: Implement minimal code to pass
4. **VERIFY**: Run the test - it MUST pass
5. **REFACTOR**: Improve if needed
6. **REPEAT**

Never write implementation before tests. Never skip the failing test verification.

### Test Types

**Unit tests**: Per function/method
- Happy path
- Error cases
- Edge cases (nil, empty, boundary)

**Integration tests**: Per feature
- Full request/response cycles
- Database operations (test DB)
- API endpoints

**Frontend tests**: Component tests
- Render correctly
- Handle user interactions
- Display loading/error states

### Test Structure

```go
func TestServiceName_MethodName_Scenario(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```

Use table-driven tests for multiple scenarios.

## Documentation Requirements

### Code Documentation

All public types, functions, and interfaces must have:
- Godoc comments explaining what it does
- Usage examples for complex APIs
- Parameter and return value descriptions

### Architecture Documentation

Update `docs/architecture.md` when:
- Adding new packages or modules
- Changing public APIs
- Adding new adapters or integrations
- Modifying data models

### User Documentation

Update `README.md` when:
- Adding new features
- Changing configuration
- Updating installation instructions

## Dependencies to Know

### Backend (Go)

- `github.com/go-chi/chi/v5` - HTTP routing
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/google/uuid` - UUID generation
- `gopkg.in/yaml.v3` - YAML parsing

### Frontend (TypeScript/React)

- `@tanstack/react-router` - Routing
- `@tanstack/react-query` - Data fetching
- `react` + `react-dom` - UI library
- `typescript` - Type safety

## When Editing This Project

1. Check `docs/architecture.md` for detailed requirements
2. Never change a public API without explicit approval
3. Never add a dependency without explicit approval
4. Never modify security boundaries without explicit approval
5. Follow TDD - tests before implementation
6. Update documentation for all changes
7. Keep PRs small and focused (~100-200 lines)

## Issue Granularity

### Epics vs Granular Issues

**Epics** (milestone-level):
- M1: Foundation
- M2: GitHub Adapter
- M3: soda Integration
- M4: Frontend
- M5: Self-Host

**Granular issues** (one per feature):
- "Implement Project repository interface"
- "Add SQLite ProjectRepository implementation"
- "Create GET /api/v1/projects endpoint"
- "Build project list page"

Each issue should be:
- Completable in one session (~100-200 lines)
- Has clear acceptance criteria
- Has DoD checklist
- Can be worked on independently

### Labels

Use these labels:

**Area:**
- `area/backend` - Go backend work
- `area/frontend` - TypeScript/React work
- `area/adapter` - Adapter implementations
- `area/api` - API endpoints
- `area/database` - Database/migrations
- `area/docs` - Documentation

**Type:**
- `type/feature` - New feature
- `type/bug` - Bug fix
- `type/refactor` - Code refactoring
- `type/test` - Test improvements
- `type/chore` - Maintenance tasks

**Priority:**
- `priority/high` - Critical path
- `priority/medium` - Important but not blocking
- `priority/low` - Nice to have

**Status:**
- `status/ready` - Ready to work on
- `status/in-progress` - Currently being worked on
- `status/blocked` - Blocked by another issue
- `status/review` - In review

**Milestone:**
- `m1/foundation`
- `m2/github-adapter`
- `m3/soda-integration`
- `m4/frontend`
- `m5/self-host`
