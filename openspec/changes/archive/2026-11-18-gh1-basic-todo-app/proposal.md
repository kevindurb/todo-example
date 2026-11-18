# Proposal

## Why

This repository is still the bare template: there is no application, the `just`
`fmt`, `lint`, `build`, and `test` recipes fail on purpose, and `openspec/specs/`
is empty. Issue #1 asks for a basic todo app so the repository becomes a real
project and the factory has something to build and deploy.

The smallest useful shape is a single Go binary that serves server-rendered HTML
and stores todos in SQLite. That keeps the runtime to one process and one data
file, needs no client JavaScript and no external service, and fits the
repository's existing `just` and container expectations.

## What Changes

- Add a Go HTTP server (`net/http`) that serves a small todo application with
  server-rendered HTML templates and no client-side JavaScript.
- Add the todo model: a required title, optional notes, a done flag, and
  created/updated timestamps. Support adding, editing, marking done/undone, and
  deleting a todo.
- Add SQLite persistence with an embedded, versioned schema, a configurable
  database path, WAL mode, a busy timeout, and parameterized queries.
- Add input validation and explicit error responses: required/oversized fields,
  not-found todos, unsupported methods, and malformed input.
- Add `just` recipes for formatting, linting, building, testing, and building
  the container image; switch `ci` to
  `pre-commit pre-push spec-validate container-build` now that the project has
  real checks and a container to verify.
- Add a multi-stage `Dockerfile` and `.dockerignore` that build a static binary
  and run it as a non-root user with the SQLite file on a volume.
- Add an `ADDR` and `DATABASE_PATH` configuration surface, a `/healthz`
  readiness endpoint, structured request logging, and graceful shutdown.
- Add tests for storage behavior and HTTP handlers, including validation,
  not-found, method, ordering, and HTML-escaping cases.
- Add a GitHub Actions container workflow that builds and publishes the image to
  GHCR on `main` and tags. This file is under `.github/`, which the implementation
  agent is not allowed to edit, so it is a maintainer-owned prerequisite recorded
  in `tasks.md`, not an apply-agent task.
- Update `README.md`, `AGENTS.md`, and project docs with the app's purpose,
  commands, configuration, and deployment procedure.

**Not in scope:** authentication or multi-user accounts, due dates, priorities,
tags, projects, search, reminders, import/export, a JSON API, client-side
JavaScript, offline support, or horizontal scaling. The app is a single-user
service meant to sit behind a private network.

## Capabilities

### New Capabilities

- `todo-management`: the todo record and its lifecycle — fields, validation,
  create/edit/delete/completion behavior, ordering, and not-found handling.
- `todo-web-ui`: the server-rendered pages and form flow — zero client
  JavaScript, post/redirect/get, validation feedback, escaping, and HTTP method
  handling.
- `todo-storage`: SQLite persistence — schema and migration, parameterized
  queries, durability settings, ordering, and database configuration.
- `todo-deployment`: the runnable artifact — container image, non-root runtime,
  environment configuration, data volume, health endpoint, graceful shutdown,
  and the GitHub Actions build/publish path.

### Modified Capabilities

None. The repository has no specs under `openspec/specs/`; `add-agentic-factory`
is a separate in-flight change and is not modified by this one.

## Impact

- New Go module and code under `cmd/` and `internal/`, plus embedded templates
  and static CSS.
- New direct dependency on `modernc.org/sqlite`, a pure-Go SQLite driver chosen
  so the container build does not need CGO or a C toolchain. This is the only
  new runtime dependency.
- New `Dockerfile`, `.dockerignore`, and container `just` recipes; `just ci`
  changes from `check-template` to the real check chain.
- New tests and test fixtures under the Go packages.
- A new `.github/workflows/container.yml` for publishing the image. It is
  control-plane infrastructure and must be added by a maintainer, not by the
  implementation agent.
- Documentation updates in `README.md`, `AGENTS.md`, and `docs/deployment.md`;
  the existing factory workflows, agents, skills, and hooks are unchanged.
