# Workspace Management — Design Notes

## Context

flux needs local copies of project repositories to:
1. **Discover pipelines** — pipeline definitions (phases, config) live in the repo itself, not in flux config
2. **Run soda** — soda operates on a local repo clone; it does not handle its own cloning

## Design Decisions

### Boundary: flux clones, soda runs

- **flux owns the clone lifecycle**: clone on project creation, pull on sync/webhook, cleanup on project deletion
- **soda runs inside the clone**: flux invokes `soda run <ticket>` with `cmd.Dir` set to the clone path
- **Pipeline discovery**: flux reads pipeline definitions from a well-known path in the repo (e.g., `.flux/pipelines.yaml` or `soda.yaml`)

### What needs building

| Component | Purpose |
|-----------|---------|
| `clone_path` on Project model | Where the repo lives on disk, derived from config + project ID |
| `clone_root` in config | Base directory for all clones, e.g. `/var/flux/repos` |
| Clone on project creation | `git clone <repo_url> <clone_path>` via GitHub App token |
| Pull on sync/webhook | `git -C <clone_path> pull` to stay current |
| Cleanup on project delete | `os.RemoveAll(<clone_path>)` |
| `cmd.Dir` on soda invocation | Set working directory to `clone_path` when running soda commands |
| Pipeline discovery from repo | Read pipeline definitions from the repo (path TBD) |

### Config sketch

```yaml
workspace:
  root: /var/flux/repos  # base path for all clones
```

### Open questions

- Should clone happen eagerly (on project creation) or lazily (on first pipeline trigger)?
- Should flux auto-pull on every webhook event, or only before pipeline runs?
- What's the well-known path for pipeline definitions? `.flux/pipelines.yaml`? `soda.yaml`?
- What about private repos — use the GitHub App installation token for `git clone`?
