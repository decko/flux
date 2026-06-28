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

### Tool Sources & Auth

The chat agent gets its tools from two sources, using flux's existing GitHub App installation tokens (no PAT needed):

```
                     ┌─ wtmcp GitHub plugin ──→ investigation/discovery
flux chat agent ─────┤     (search, my_work, PR details, files, reviews, comments)
  (MCP client)       │
                     └─ flux tool registry ────→ creation/execution
                           (create_issue, trigger_pipeline, sync, audit)
```

**wtmcp** provides 13 read tools + 3 write tools (PR reviews, comments). It handles the GitHub API interaction, caching, rate limiting, and SSRF protection. The chat agent connects to it as an MCP client over stdio.

**flux** provides the tools wtmcp doesn't cover: creating issues (the GitHub plugin lacks `create_issue`), triggering pipelines, managing sync, and querying the audit trail. These wrap existing domain services.

**Token flow:** `appAuth.GetToken(ctx, project.InstallationID)` at session start → passed to wtmcp via `GITHUB_TOKEN` env var on process start. Per-session wtmcp process with fresh token. Session restart if >55 minutes (installation tokens expire after 1 hour).

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
│ - Manages SSE streaming connection               │
└──────────────────┬───────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────┐
│ LLM Orchestrator                                 │
│ - Sends messages + tool definitions to LLM       │
│ - Parses tool call responses                     │
│ - Feeds tool results back to LLM (tool loop)     │
└──────┬───────────────────────────────┬───────────┘
       │                               │
       ▼                               ▼
┌──────────────┐              ┌──────────────────┐
│ LLM Client   │              │ Tool Registry    │
│ - OpenAI     │              │ - list_tickets   │
│ - Anthropic  │              │ - create_issue   │
│ (adapter     │              │ - trigger_pipeline│
│  pattern)    │              │ - ...            │
└──────────────┘              └──────┬───────────┘
                                     │
                                     ▼
                          ┌──────────────────────┐
                          │ Existing Services    │
                          │ - TicketService      │
                          │ - PipelineService    │
                          │ - ProjectService     │
                          │ - SyncService        │
                          │ - AuditService       │
                          └──────────────────────┘
```

### Key design decisions

| Decision | Rationale |
|----------|-----------|
| **LLM adapter pattern** | Matches existing `internal/adapter/ticket/` and `internal/adapter/scm/` patterns. First impl: OpenAI. Anthropic follows same interface. |
| **Tool registry as domain service** | Tools map 1:1 to existing domain service methods. The registry just describes them in LLM-friendly schemas. |
| **SSE for streaming** | Stdlib `http.Flusher`. No WebSocket library needed. Simpler than WebSocket for unidirectional server→client streaming. |
| **No persistence for chat (MVP)** | Ephemeral, per-request. No session tables, no message store, no retention policy. Add in Phase 2 if needed. |
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

### Read-only tools (Phase 1)

```json
{
  "name": "list_tickets",
  "description": "List tickets across all projects or filter by project and status",
  "parameters": {
    "project_id": { "type": "string", "description": "Project ID to filter by" },
    "status": { "type": "string", "enum": ["open", "closed"] }
  }
}
```

### Write tools (Phase 1, with confirmation)

```json
{
  "name": "create_issue",
  "description": "Create a new GitHub issue in a project's repository. Requires user confirmation.",
  "parameters": {
    "project_id": { "type": "string", "required": true },
    "title": { "type": "string", "required": true },
    "body": { "type": "string" },
    "labels": { "type": "array", "items": { "type": "string" } }
  }
}
```

### Admin tools (Phase 2+, with re-authentication)

```json
{
  "name": "trigger_sync",
  "description": "Trigger a manual sync for a project. Admin only. Requires re-authentication.",
  "parameters": {
    "project_id": { "type": "string", "required": true }
  }
}
```

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
