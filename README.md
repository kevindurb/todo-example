# todo-example

A single-user todo app: one Go binary, server-rendered HTML, and a local SQLite
database. It has no client-side JavaScript, no external database, and no
services to run alongside it.

The app is meant to sit on a private network or behind an authenticating proxy.
It has no accounts or authorization of its own, so do not expose it directly to
the public internet. See [`docs/deployment.md`](docs/deployment.md).

## Quick start

```sh
just run
```

Then open <http://localhost:8080>. Todos are stored in `./data/todos.db`, which
is created on first start.

## What it does

- Add a todo with a required title (up to 200 characters) and optional notes (up
  to 2,000 characters).
- Edit a todo's title and notes, mark it done or reopen it, and delete it after
  a confirmation step.
- Lists open todos before done ones, newest first within each group.
- Serves `GET /healthz` for readiness checks and shuts down gracefully on
  `SIGINT`/`SIGTERM`.

## Commands

| Command | Does |
|---|---|
| `just` | List recipes |
| `just fmt` | Format check with `gofmt`; fails when a file is unformatted |
| `just lint` | `go vet ./...` |
| `just tidy` | `go mod tidy` |
| `just build` | Build `bin/todo` |
| `just test` | Run the Go test suite |
| `just run` | Run the app locally |
| `just container-build` | Build the container image as `todo-example:local` |
| `just container-run` | Run the image with a named `todo-data` volume |
| `just container-test` | Opt-in acceptance test: build the image and check persistence across a restart |
| `just pre-commit` | `fmt lint` |
| `just pre-push` | `build test` |
| `just ci` | What CI runs: `pre-commit pre-push spec-validate container-build` |
| `just spec-list` | List OpenSpec changes |
| `just spec-validate` | Validate every spec and change |
| `just install-hooks` | Point git at `.githooks` (once per clone) |

Run `just ci` before reporting work complete. Do not bypass hooks with
`--no-verify`.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `ADDR` | `:8080` | Listen address, in `host:port` form |
| `DATABASE_PATH` | `./data/todos.db` | SQLite file; missing parent directories are created |

Startup fails with a message naming the bad value when `ADDR` or
`DATABASE_PATH` cannot be used.

## Deployment

The repository builds a multi-stage image that runs as a non-root user from a
minimal runtime, with `/data` as the only writable path. Build it with
`just container-build` and run it with a volume mounted at `/data`; see
[`docs/deployment.md`](docs/deployment.md) for the full procedure, health check,
image tags, and backup notes.

## Development

The app is a Go module at `github.com/kevindurb/todo-example`:

| Path | Contents |
|---|---|
| `cmd/todo` | Configuration, server wiring, signals, and graceful shutdown |
| `internal/todo` | The todo record and validation rules |
| `internal/store` | SQLite schema, migrations, and queries |
| `internal/web` | Routes, handlers, embedded templates, and stylesheet |

SQLite is a single-writer store: run one process per database file. The store
opens one connection (`SetMaxOpenConns(1)`), uses WAL journaling, and sets a busy
timeout so writes serialize cleanly.

Feature work goes through the repository's OpenSpec change lifecycle; see
[`AGENTS.md`](AGENTS.md) and [`docs/factory.md`](docs/factory.md).
