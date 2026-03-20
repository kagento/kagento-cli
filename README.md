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

# During a session, check your progress
kagento check <session_id>

# Happy with your score? Submit.
kagento submit <session_id>
```

## Commands

| Command | Description |
|---------|-------------|
| `kagento login` | Log in via browser (OAuth 2.0 device flow) |
| `kagento logout` | Clear saved credentials |
| `kagento whoami` | Show current user |
| `kagento check <session_id>` | Run tests and preview your score without ending the session |
| `kagento submit <session_id>` | Submit your solution -- runs final tests, stops the session, records your score |
| `kagento status <session_id>` | Show session status, task name, and timing |
| `kagento auth` | Authenticate with the Docker registry (for task authors) |
| `kagento push <slug>` | Push task + test images to the registry (for task authors) |

## Typical session flow

```
$ kagento login
Open https://kagento.io/... and enter code: ABCD-EFGH
Waiting for login...
Logged in as alice

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
| `KAGENTO_API` | `http://localhost:8080/v1/graphql` | GraphQL endpoint |
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
