# Self-Host Workflow

Flux manages its own software development lifecycle. This guide describes the full loop.

## Architecture

```
GitHub Issues/PRs ←→ flux (sync) ←→ soda (orchestrator) ←→ AI agents
       ↑                                                    ↓
       └──────────── code changes ← pull requests ←─────────┘
```

## The Loop

1. **Create an issue** on flux's GitHub repo
2. **flux syncs** the issue via the GitHub adapter
3. **Trigger soda** on the ticket from the flux UI
4. **soda dispatches** AI agents (planner, coder, reviewer, QE)
5. **Agents write code**, run tests, create a pull request
6. **flux syncs the PR** via the GitHub adapter
7. **Human reviews** the PR on GitHub and merges
8. **Close the issue** — flux updates ticket status on next sync

## Setup

1. Follow the [quickstart](../README.md#quickstart-self-host)
2. Configure `flux.yaml` with your GitHub owner/repo
3. Install [soda](https://github.com/decko/soda) on your PATH
4. Set `GITHUB_TOKEN` and `JWT_SECRET` environment variables

## Daily Use

- **Dashboard**: View project/ticket/PR counts at a glance
- **Tickets**: Browse synced issues with relationship detection
- **Pipeline Runs**: Trigger and monitor soda runs per ticket
- **Pull Requests**: Track PRs linked to tickets with review status

## Configuration Reference

See [flux.yaml.example](../flux.yaml.example) for all configuration options.
