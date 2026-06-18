# Flux MVP Roadmap

## Goal

Flux manages its own development. Add the flux repo, see tickets, trigger soda, see PRs.

## Milestones

### M1: Foundation

- [ ] Go module + Chi server + SQLite
- [ ] Domain models (Project, Ticket, PR, PipelineRun)
- [ ] Repository interfaces + SQLite implementation
- [ ] Database migrations
- [ ] Basic REST API (CRUD projects, tickets, PRs)
- [ ] Frontend SPA scaffold (Vite + TS + TanStack)
- [ ] Embed frontend in Go binary

### M2: GitHub Adapter

- [ ] GitHub ticket adapter (issues)
- [ ] GitHub PR adapter
- [ ] Webhook receiver for real-time sync
- [ ] Polling fallback
- [ ] Relationship auto-detection (issue refs, labels)
- [ ] Sync service (background job)

### M3: soda Integration

- [ ] soda orchestrator adapter
- [ ] Trigger soda runs via subprocess/JSON-RPC
- [ ] Monitor run status
- [ ] Store run results (phases, cost, duration)
- [ ] API: trigger run, get status, list runs

### M4: Frontend

- [ ] Project list + detail page
- [ ] Ticket list + detail page
- [ ] Ticket relationship visualization
- [ ] PR list linked to tickets
- [ ] Trigger soda button on ticket
- [ ] Pipeline run status view
- [ ] Basic auth (login)

### M5: Self-Host

- [ ] Add flux as a project in flux
- [ ] Sync flux GitHub issues + PRs
- [ ] Trigger soda on flux tickets from flux UI
- [ ] View soda run results in flux
- [ ] Deploy flux (single binary)

## Success Criteria

1. Flux runs as a single binary
2. Flux GitHub repo is added as a project
3. Issues and PRs are visible in the UI
4. soda can be triggered on a ticket from the UI
5. Pipeline run status is visible in the UI
6. PRs are linked to their tickets

## Post-MVP

- Multi-user + roles
- Jira/Linear adapters
- WebSocket live updates
- Agent worker registration (JSON-RPC)
- MCP server for agent write-back
- Project onboarding agent
- Cost tracking dashboard
- Ticket relationship graph visualization
