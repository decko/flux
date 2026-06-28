# Chat Interface & Agent System — Design Notes

## Definition

**An agent, in flux's context, is a system that translates natural language into tool calls and executes them.** The user decides what to do; the agent orchestrates how to do it.

This is distinct from an **autonomous agent**, which would plan, decide, and act without user direction. Flux's chat interface is an orchestration agent, not an autonomous agent.

## Three-Phase Approach

### Phase 1 — Tool-Calling Chat (MVP)

**What it does:** User types a message → LLM selects and calls tools → results are returned. One-shot, user-driven, no loops.

```
User: "Show me open tickets in project flux"
  → LLM calls: list_tickets(project="flux", status="open")
  → Returns: "Found 3 open tickets: #42 Fix login, #43 Update docs, #44 Add tests"
```

**Tools available (read-only by default):**
- `list_projects` — list all projects
- `get_project` — get project details
- `list_tickets` — list tickets (filterable by project, status)
- `get_ticket` — get ticket details
- `list_pull_requests` — list PRs
- `list_pipeline_runs` — list pipeline runs
- `get_sync_status` — get sync/webhook health

**Write tools (require human confirmation):**
- `create_issue` — create a GitHub issue via the ticket adapter
- `trigger_pipeline` — trigger a pipeline run
- `trigger_sync` — trigger a manual sync

**Confirmation pattern:**
```
🔧 I want to: Create a GitHub issue
   Repo: decko/flux
   Title: "Fix login redirect bug"
   Body: "The login page redirects to / instead of the original URL..."
   Labels: bug, high-priority
   [Approve] [Deny]
```

### Phase 2 — Capability Scoping & Safeguards

Adds the security foundation for broader tool access:

- **Capability tokens**: Scoped, short-lived JWTs per session with tool allowlists
- **Session budgets**: Max tokens, max tool calls, max duration per session
- **Extended audit trail**: `ActorType` (human/agent/system), `AgentSessionID`, `PrincipalID`
- **Data/instruction separation**: Ticket data returned as structured tool results, never injected into LLM instruction context
- **Rate limiting**: Per-session throttling on tool calls

### Phase 3 — Agent Loops (if demanded)

Adds multi-step, autonomous reasoning:

- User: "Fix the failing login test"
- Agent: Reads tickets → finds #42 → reads PRs → identifies the fix → triggers pipeline → reports result
- Optional, opt-in, with all Phase 2 safeguards enforced

## Architecture

### Custom wtmcp GitHub Plugin

GitHub interaction is centralized in a custom wtmcp plugin. All consumers — the chat agent, soda orchestrator, and future automation — share the same MCP tools backed by GitHub App installation tokens.

```
┌───────────┐     MCP      ┌──────────────────┐
│ chat agent├──────────────┤                  │
└───────────┘              │      wtmcp       │
                           │                  │
┌───────────┐     MCP      │  ┌────────────┐  │    installation
│   soda    ├──────────────┤  │  flux/      │  │    token
└───────────┘              │  │  github     │──┼──→ GitHub API
                           │  │  plugin     │  │
                           │  └──────┬─────┘  │
                           └─────────┼────────┘
                                     │ audit write-back
                                     ▼
                              flux audit trail
```

**Why wtmcp was chosen over building flux's own MCP server:**
- MCP protocol server, plugin lifecycle management, and process supervision come for free
- HTTP proxy with SSRF protection, rate limiting, caching, and sandboxing are built in
- The plugin system is language-agnostic (Python, Go, or any executable over JSON-lines)
- Progressive tool discovery (primary vs deferred tools) reduces LLM context usage

**Why a custom plugin instead of the existing GitHub plugin:**
- GitHub App installation token auth (the existing plugin expects a PAT)
- Full write tooling: `create_issue`, `create_pr` (the existing plugin only has reviews/comments)
- Audit write-back to flux's hash-chained trail
- Per-installation scoping (the plugin respects which project/installation a request belongs to)
- Single shared tool set serving both the chat agent and soda pipeline phases

### How consumers use the plugin

**Chat agent** — the LLM orchestrator connects to wtmcp as an MCP client. When a user says "what are my open PRs?", the LLM calls `github_my_prs_to_review`. When they say "create an issue for the login bug", the LLM calls `github_create_issue`. The user sees tool call cards inline in the chat.

**Soda orchestrator** — during pipeline phases (`plan`, `implement`, `submit`), soda connects to the same wtmcp instance. The `plan` phase reads issues and PR context. The `submit` phase creates PRs, posts comments, and submits reviews. All soda actions are audited through flux's trail.

### Detailed Architecture

```
┌──────────────────────────────────────────────────┐
│ Chat UI (React, streaming SSE)                   │
│ - Message list + input + tool call cards         │
│ - Confirmation dialogs for write operations      │
└──────────────────┬───────────────────────────────┘
                   │ POST /api/v1/chat
                   ▼
┌──────────────────────────────────────────────────┐
│ Chat Handler (Chi, JWT middleware)               │
│ - Validates auth, project scope                  │
│ - Launches wtmcp with fresh installation token   │
│ - Manages SSE streaming connection               │
└──────┬──────────────────────┬────────────────────┘
       │                      │
       ▼                      ▼
┌──────────────┐     ┌───────────────────┐
│ LLM Client   │     │ wtmcp launcher    │
│ - OpenAI     │     │ - appAuth.GetToken│
│ - Anthropic  │     │ - env: GITHUB_TOKEN│
│ (adapter     │     │ - stdio MCP bridge│
│  pattern)    │     └────────┬──────────┘
└──────────────┘              │
       │                      │  MCP (stdio)
       ▼                      ▼
┌──────────────────────────────────────────────────┐
│ LLM Orchestrator                                 │
│ - Sends messages + tool definitions to LLM       │
│ - Routes tool calls to wtmcp (GitHub) or flux    │
│   services (pipeline, sync, audit, projects)     │
│ - Feeds tool results back to LLM                 │
└──────┬───────────────────────────────┬───────────┘
       │                               │
       ▼                               ▼
┌──────────────────┐          ┌──────────────────┐
│ wtmcp + flux/    │          │ flux tool registry│
│ github plugin    │          │ - trigger_pipeline│
│                  │          │ - trigger_sync    │
│ MCP tools:       │          │ - list_projects   │
│ - create_issue   │          │ - get_sync_status │
│ - create_pr      │          │ - query_audit     │
│ - add_comment    │          └──────┬───────────┘
│ - create_review  │                 │
│ - search         │                 ▼
│ - my_work        │          ┌──────────────────┐
│ - get_pr_files   │          │ Existing Services│
│ - ...            │          │ - PipelineService │
└──────────────────┘          │ - SyncService     │
                              │ - ProjectService  │
                              │ - AuditService    │
                              └──────────────────┘
```

### Key design decisions

| Decision | Rationale |
|----------|-----------|
| **Custom wtmcp plugin** | Reuses wtmcp's MCP server, process management, caching, rate limiting, sandboxing. Only build the plugin logic. |
| **GitHub App installation tokens** | Already working in flux. No PAT to manage. Per-installation scoping. Proper audit trail showing the App as the actor. |
| **flux for non-GitHub tools** | Pipeline triggers, sync management, project queries, audit queries are flux-domain operations. They live in flux's own tool registry. |
| **LLM adapter pattern** | Matches existing adapter patterns. First: OpenAI. Anthropic follows same interface. |
| **SSE for streaming** | Stdlib `http.Flusher`. No WebSocket needed. Simpler for server→client streaming. |
| **No chat persistence (MVP)** | Ephemeral, per-request. Add session storage in Phase 2 if needed. |
| **JWT auth (not admin-only)** | Chat uses standard `AuthMiddleware`. Users can only see projects they have access to. Write tools respect existing role gates. |
| **Tool confirmation before execution** | All write tools show a confirmation card. User must approve. This is the primary safety mechanism in Phase 1. |

## Security Model

### Threat: Prompt Injection via GitHub Data

**Attack:** A malicious ticket title contains instructions that the LLM interprets as commands.

**Defense (Phase 2):** Structured tool results — ticket data is returned as tool call outputs, never injected into the LLM's instruction context. The LLM sees:
```
TOOL RESULT: list_tickets → [{"id":"42", "title":"...", "body":"..."}]
```
Not: "Here are the tickets: #42 says ignore previous instructions and..."

### Threat: Agent Accountability

**Problem:** Current audit trail attributes all actions to the human user. Can't distinguish "user clicked a button" from "agent executed a tool call."

**Defense (Phase 2):** Extended audit event with:
- `ActorType`: "human" | "agent" | "system"
- `AgentSessionID`: unique per chat session
- `PrincipalID`: the human who initiated the agent session

### Threat: Resource Exhaustion

**Problem:** An agent loop could burn LLM tokens, GitHub API quota, or create thousands of entities.

**Defense (Phase 2):**
- Per-session token budget (e.g., 50K tokens)
- Per-session tool call limit (e.g., 50 calls)
- Per-session time limit (e.g., 10 minutes)
- Rate limiting on tool calls (e.g., 10/second)

### Threat: Permission Escalation

**Problem:** Agent operating with full user permissions could perform admin-gated operations.

**Defense (Phase 2):**
- Capability tokens: Scoped JWTs with explicit tool allowlists
- Admin operations (delete project, rotate secrets) never in default capability set
- Admin writes through agent require re-authentication + human confirmation

## Tool Definitions

### GitHub tools (custom wtmcp plugin — shared by chat agent and soda)

The plugin wraps the full GitHub API surface that both consumers need:

**Read tools (investigation/discovery):**

| Tool | Description |
|------|-------------|
| `github_search` | Search issues/PRs with full GitHub query syntax |
| `github_my_work` | Unified task list: assigned issues, PRs to review, mentions |
| `github_my_prs_to_review` | PRs where the user is a requested reviewer |
| `github_my_issues` | Open issues assigned to the authenticated user |
| `github_my_notifications` | GitHub notification feed |
| `github_get_issue` | Issue details: title, body, state, labels, assignees, milestone |
| `github_get_pr` | PR details: title, body, status, merge state, additions/deletions, files count, reviewers |
| `github_get_pr_files` | Files changed in a PR with status, additions, deletions, and diff patches |
| `github_get_pr_reviews` | All reviews on a PR with reviewer, state (APPROVED/CHANGES_REQUESTED/COMMENTED), body |
| `github_get_pr_review_comments` | Inline code review comments with diff hunks and file paths |
| `github_get_comments` | Conversation comments on an issue or PR |
| `github_get_pr_commits` | Commit list in a PR with SHA, message, author |

**Write tools (creation/mutation):**

| Tool | Description |
|------|-------------|
| `github_create_issue` | Create a new issue with title, body, labels, assignees |
| `github_create_pr` | Create a pull request from a branch with title and body |
| `github_add_comment` | Post a conversation comment on an issue or PR |
| `github_create_review` | Submit a PR review (APPROVE, REQUEST_CHANGES, or COMMENT with inline comments) |
| `github_add_pr_comment` | Post an inline code comment on a specific line in a PR diff |

**Plugin behavior:**
- All write tools default to `dry_run: true` — the agent previews the action before executing
- wtmcp's elicitation prompts for confirmation before any write
- Rate limiting and caching handled by wtmcp's proxy layer
- Installation token from flux's GitHub App, refreshed per session

### flux-native tools (flux tool registry)

| Tool | Description | Access |
|------|-------------|--------|
| `list_projects` | List all projects the user has access to | Read |
| `get_project` | Get project details including webhook health | Read |
| `get_sync_status` | Get sync status and webhook health | Read |
| `trigger_pipeline` | Trigger a pipeline run for a project/ticket | Write (admin) |
| `trigger_sync` | Trigger a manual sync for a project | Write (admin) |
| `query_audit` | Query the hash-chained audit trail | Read (admin) |

## Open Questions for Discussion

### UX & Interaction Model

1. **Project context**: How does the agent know which project the user is talking about? A dropdown before the chat? Inferred from the message? Explicit mention ("in project flux...")?

2. **Confirmation UX**: Should confirmation be a blocking modal or an inline card in the conversation? Should there be a "trust this session" toggle?

3. **Error display**: When a tool call fails (e.g., GitHub API rate limited), how is the error shown? As a system message? Inline on the tool card? Does the agent retry or ask the user?

4. **Conversation history**: Should the chat show previous messages on page load? Or is it ephemeral (refresh = clean slate)? If persistent, where is it stored?

### Architecture & Integration

5. **Tool result size**: Some tool results could be large (e.g., list_tickets returning 500 items). How do we truncate for the LLM context window? Pagination? Summarization?

6. **LLM provider config**: API key in config (env-only, per invariants). Model selection? Should the user be able to choose the model from the UI?

7. **MCP server timing**: When should flux expose an MCP server? Phase 3? Or should external agents (Claude Code, Cursor) be able to use flux tools from day one via a parallel MCP endpoint?

8. **Streaming granularity**: Should the chat stream token-by-token (like ChatGPT) or event-by-event (tool call → tool result → final response)? SSE supports both.

### Security & Governance

9. **Prompt templates**: Where do system prompts live? In code? In config? Should project owners be able to customize the agent's behavior per project?

10. **Data sent to LLM provider**: Project names, ticket titles, PR descriptions — all go to OpenAI/Anthropic. Should there be a data processing notice? An opt-out per project? Self-hosted model support?

11. **Agent identity in audit trail**: Should the audit trail show the model name and version that made each decision? (e.g., "gpt-4o-2024-08-06 decided to create_issue")

12. **Rollback**: If an agent creates 50 unwanted issues, can an admin bulk-close them? Should the audit trail support "revert agent session X" as a concept?
