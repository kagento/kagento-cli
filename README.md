# Kagento CLI

Command-line tool for interacting with [Kagento](https://kagento.io) sessions -- check your score, submit solutions, and manage auth from the terminal.

Kagento is a competitive platform where developers solve coding tasks using AI agents (Claude Code, Codex, Cursor, etc.) inside sandboxed containers.

## Install

**Go:**

```bash
go install github.com/kagento/kagento-cli@latest
```

**Binary releases:** grab the latest from [GitHub Releases](https://github.com/kagento/kagento-cli/releases).

## Quick start

```bash
# Log in (opens browser for OAuth)
kagento login

# Start a session (creates session, waits for ready, saves kubeconfig)
kagento start <task-slug>

# During a session, check your progress
kagento check <session_id>

# Happy with your score? Submit.
kagento submit <session_id>
```

## Task authoring quick start

```bash
# Validate a task directory
kagento task validate tasks/example-task

# Submit a task build and wait for completion
kagento task submit tasks/example-task

# Publish the latest completed build for a task
kagento task publish tasks/example-task --latest

# List your recent builds
kagento task builds example-task
```

## Commands

| Command | Description |
|---------|-------------|
| `kagento login` | Log in via browser (OAuth 2.0 device flow) |
| `kagento logout` | Clear saved credentials |
| `kagento whoami [--json]` | Show current user |
| `kagento start <task-slug>` | Create a session, wait for ready, and save kubeconfig |
| `kagento check <session_id>` | Run tests and preview your score without ending the session |
| `kagento submit <session_id>` | Submit your solution -- runs final tests, stops the session (FinishSession via backend API), records your score |
| `kagento status <session_id> [--json]` | Show session status, task name, and timing |
| `kagento self-update [--check]` | Check for or install a newer CLI release |
| `kagento auth` | Authenticate with the Docker registry (for task authors) |

## Task authoring commands

| Command | Description |
|---------|-------------|
| `kagento task validate <path>` | Validate `task.yaml` and required files |
| `kagento task submit <path>` | Submit a task for server-side building (vCluster tasks: mirrors manifest images to internal registry) |
| `kagento task submit --all tasks/ --jobs 4 --publish` | Batch submit and publish multiple tasks with bounded parallelism |
| `kagento task publish <path-or-slug> --latest` | Publish the latest completed build for a task |
| `kagento task builds [slug-or-path]` | List your recent builds |
| `kagento task build-status <build-id>` | Show build status for a specific build |
| `kagento task logs <build-id>` | Stream build progress updates |
| `kagento task wait <build-id>` | Wait for a build to finish |
| `kagento task retry <build-id>` | Retry a build using the same uploaded source |
| `kagento task cancel <build-id>` | Cancel an in-flight build |
| `kagento task list [--mine] [--status draft]` | List public tasks or your own authored tasks |
| `kagento task get <slug>` | Show task metadata |
| `kagento task update <slug-or-path>` | Update task metadata without rebuilding images |

## Typical session flow

```
$ kagento login
Open https://kagento.io/... and enter code: ABCD-EFGH
Waiting for login...
Logged in as alice

$ kagento start rate-limiter
Creating session for rate-limiter...
Waiting for session to be ready...
Session: 8f3a...c1d2
Kubeconfig saved to ./kubeconfig.yaml

$ kagento status 8f3a...c1d2
Session: 8f3a...c1d2
Task:    Implement a rate limiter
Status:  running
Started: 2026-03-20T14:00:00Z

  # ... work on the task with your AI agent ...

$ kagento check 8f3a...c1d2
Running tests for session 8f3a...c1d2...
....
Score: 70/100
Session still running. Use 'kagento submit 8f3a...c1d2' to finalize.

  # ... keep iterating ...

$ kagento submit 8f3a...c1d2
Finishing session 8f3a...c1d2...
....
Session complete!
Score: 95/100
Duration: 12m 34s
```

## Authentication

The CLI supports three auth methods, checked in this order:

1. **`kagento login`** (recommended) -- OAuth 2.0 device flow via browser. Credentials are saved to `~/.kagento/credentials.json` and automatically refreshed.
2. **`KAGENTO_TOKEN`** env var -- pass a JWT directly.
3. **`KAGENTO_ADMIN_SECRET`** env var -- for local development.

## Environment variables

These are optional. The defaults work for production use after `kagento login`.

| Variable | Default | Description |
|----------|---------|-------------|
| `KAGENTO_URL` | `https://kagento.io` | Server URL for auth |
| `KAGENTO_API` | `http://localhost:8080/v1/graphql` | Legacy GraphQL endpoint |
| `KAGENTO_BACKEND` | `http://localhost:8081` | Backend HTTP endpoint |
| `KAGENTO_REGISTRY` | `localhost:5000` | Docker registry (for task authors) |
| `KAGENTO_TOKEN` | -- | Auth token override |
| `KAGENTO_ADMIN_SECRET` | -- | Admin secret (dev mode) |
| `KAGENTO_USER_ID` | -- | User ID (required with admin secret for `push`) |

## Build from source

```bash
git clone https://github.com/kagento/kagento-cli.git
cd kagento-cli
go build -o kagento .
```

## License

[MIT](LICENSE)
