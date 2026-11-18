# Design

## Context

See `proposal.md` for motivation and `specs/` for the behavior contract. The
repository is the `base` template: it has a `just` command contract whose
`fmt`, `lint`, `build`, and `test` recipes are failing stubs, no Go module, and
no application code. The factory control plane already exists, and its
implementation agent may edit project code but not `.github/`, `.opencode/`,
`.agents/`, `.githooks/`, or `.git/`.

The application is a single-user service. It has no accounts, sessions, or
authorization, and it is expected to run on a private network or behind an
authenticating proxy. That constraint keeps the first version small; it is not
a claim that the app is safe to expose directly to the public internet.

Assumptions recorded for the issue's underspecified parts:

- "Basic fields" means a required title, optional notes, a done flag, and
  created/updated timestamps. Due dates, priorities, tags, and projects are out
  of scope.
- "Deployable from a container built from GHA" means a multi-stage image
  published to GHCR by a GitHub Actions workflow, plus documented
  `docker run`/volume/health usage.
- The container workflow is control-plane infrastructure. The implementation
  agent cannot edit `.github/`, so the workflow is a maintainer-owned
  prerequisite recorded outside the agent's tracked tasks.

## Goals / Non-Goals

**Goals:**

- One self-contained binary with no CGO, no external database, and no client
  JavaScript.
- A small, testable split between the model, SQLite store, and HTTP layer.
- A container that runs as a non-root user, writes only to its data volume, and
  exposes a readiness check.
- Real `just` checks that run unchanged locally and in the existing `CI`
  workflow.
- A deployment path that publishes a verifiable image without adding runtime
  secrets.

**Non-Goals:**

- Authentication, multi-user data isolation, CSRF tokens for a session model, or
  TLS termination in the app.
- A JSON API, a SPA, offline behavior, or browser-side state.
- Migrations from a previous application schema; this is the first schema.
- Multi-replica deployment. SQLite is a single-writer store; the app assumes one
  process per database file.

## Decisions

### 1. Go standard library HTTP and `html/template`

Use `net/http` with `http.ServeMux` and `html/template` rather than a web
framework or a template engine dependency. The route set is small, the standard
library provides method-aware patterns and escaping, and a framework would add a
maintenance commitment without changing user-visible behavior.

Alternatives considered: Chi or Echo (routing helpers not needed for a handful
of routes); `templ` (a code-generation step and another dependency); hand-written
string HTML (loses contextual escaping).

### 2. Pure-Go SQLite via `modernc.org/sqlite`

Use `modernc.org/sqlite` so `CGO_ENABLED=0` produces a static binary and the
container needs no C toolchain, libc, or build stage at runtime. The driver is
the only new direct dependency. The store owns the `*sql.DB` and exposes typed
methods; handlers never write SQL.

Alternatives considered: `mattn/go-sqlite3` (requires CGO and a C compiler, which
complicates the static image); an embedded key-value store (loses SQL and
relational queries); PostgreSQL (an external service is disproportionate for a
single-user todo list).

### 3. Server-rendered pages with post/redirect/get and no JavaScript

Every mutation is a normal form POST followed by a `303 See Other` redirect to
the list. Delete uses a dedicated confirmation page with a POST form, so
destruction is deliberate without a JavaScript `confirm()`. Completion uses an
explicit desired state (`done=true` or `done=false`) rather than a blind toggle,
so a repeated submission is idempotent.

Alternatives considered: a `fetch`-driven UI (violates the zero-JavaScript
requirement); a direct delete link (a prefetch or crawler could delete data); a
toggle endpoint (a retry flips the state twice).

### 4. Embedded templates and stylesheet

Keep templates under `internal/web/templates/` and CSS under
`internal/web/static/`, embedded with `embed.FS`. The binary is then
self-contained and the container does not need a separate asset copy. Templates
are parsed once at startup and executed with a shared layout.

Alternatives considered: files on disk next to the binary (adds a deployment
copy step and drift); inline HTML strings (harder to review and escape).

### 5. Explicit schema version with `PRAGMA user_version`

On open, apply an embedded schema in a transaction and set `user_version` to
`1`. Startup fails if the recorded version is newer than the binary understands.
This gives a first-version migration hook without a migration library. The
schema is one `todos` table plus an index supporting the list order.

Alternatives considered: `CREATE TABLE IF NOT EXISTS` only (no version record,
so the next schema change has no safe starting point); a migration framework
(overhead for one table).

### 6. One writer connection, WAL, and a busy timeout

Open SQLite with WAL journaling, a bounded busy timeout, foreign keys enabled,
and `SetMaxOpenConns(1)` so writes are serialized in-process. This avoids
`database is locked` surprises for a single-user service while keeping reads
simple. The store returns a distinct not-found error instead of a zero value.

Alternatives considered: a larger connection pool (more lock contention and no
benefit at this scale); an external queue (unnecessary).

### 7. Package split: `cmd/todo`, `internal/todo`, `internal/store`, `internal/web`

`internal/todo` holds the record and validation rules, `internal/store` owns
SQLite and the schema, `internal/web` owns handlers, routes, templates, and
middleware, and `cmd/todo` wires configuration, storage, and the server. This
keeps the model testable without a database and the handlers testable without a
real network listener.

Alternatives considered: a single `main` package (harder to test and to keep the
SQL boundary); a repository interface with mocks (the SQLite store is fast
enough to use directly in tests).

### 8. Configuration from `ADDR` and `DATABASE_PATH`

Defaults are `:8080` and `./data/todos.db`; the container sets
`DATABASE_PATH=/data/todos.db` and mounts `/data` as a volume. Startup creates
missing parent directories, validates the listen address by parsing it, and
fails fast with a message naming the bad value. There is no config file and no
secret configuration.

Alternatives considered: flags only (harder to use in a container); a config
file (another format and failure mode for two values).

### 9. Container: multi-stage build, distroless static runtime, non-root

Build with `golang:<version>-alpine` and `CGO_ENABLED=0`, then copy the binary
and an empty `/data` directory into `gcr.io/distroless/static-debian12:nonroot`.
The runtime has no shell or package manager, runs as the non-root user, exposes
`8080`, and relies on the data volume for writes. The SQLite driver is pure Go,
so no libc is needed.

Alternatives considered: an Alpine runtime (needs a shell and package updates);
a scratch image (no CA certificates or non-root user setup); running as root (a
needless privilege).

### 10. Health check pings the database

`GET /healthz` performs a short-timeout `Ping` on the store and returns `200`
only on success, `503` otherwise. The response body is small text, not JSON, so
the check has no content-negotiation edge cases. The container `HEALTHCHECK` is
not used because the orchestrator or deployment platform should own liveness;
the endpoint is the portable contract.

Alternatives considered: a static `200` (would not catch a broken database); a
separate metrics endpoint (out of scope).

### 11. Graceful shutdown with a bounded drain

`cmd/todo` listens for `SIGINT`/`SIGTERM`, calls `Shutdown` with a 10-second
context, then closes the store. A second signal exits immediately; SQLite's WAL
and transactional writes keep the database consistent.

Alternatives considered: exiting on the first signal (drops in-flight requests);
an unbounded drain (can hang a deployment).

### 12. The GitHub Actions container workflow is maintainer-owned

The change needs a `.github/workflows/container.yml` that builds on pull
requests and publishes to GHCR on `main` and version tags, using `GITHUB_TOKEN`
and `packages: write`. The apply agent is forbidden from editing `.github/` and
must report such work as blocked, so the workflow is listed as a maintainer
prerequisite rather than an agent task. The app-side work still delivers the
Dockerfile, `just container` recipes, and a `just ci` that builds the image
locally, which the existing `CI` workflow exercises.

Alternatives considered: publishing from `just ci` (the existing workflow has
`contents: read` and no registry login, so it cannot push); putting the
workflow in the implementation PR (the apply guard rejects it and blocks the
whole phase); skipping publishing (fails the acceptance criterion).

## Risks / Trade-offs

- **The app has no authentication** → run it on a private network or behind an
  authenticating proxy; document this prominently. Adding accounts is a later
  change, not a hidden assumption.
- **SQLite is a single-writer store** → one process per database file, a busy
  timeout, and `SetMaxOpenConns(1)`; multi-replica deployment is explicitly out
  of scope.
- **The container workflow is outside the apply agent's write scope** → the
  maintainer adds it as reviewed control-plane infrastructure before archiving;
  the tasks and report name it so it cannot be forgotten.
- **`modernc.org/sqlite` increases binary size** → acceptable for a small
  service; it buys a CGO-free, static image.
- **The template's `just ci` currently runs `check-template`** → switch it to
  `pre-commit pre-push spec-validate` in this change, or the new checks would
  never run in CI.
- **No CSRF tokens** → the app has no session or cookie-based authority, so a
  cross-site form post can only do what an unauthenticated visitor could do.
  Revisit if authentication is added.
- **Large request bodies** → cap request bodies with `http.MaxBytesReader` and
  validate field lengths before touching the store.

## Migration Plan

1. Land the proposal, then implement the Go module, store, web layer, tests,
   `just` recipes, container files, and docs.
2. A maintainer adds `.github/workflows/container.yml` as a control-plane
   change before the archive phase. Verify it with `actionlint` and a pull
   request build.
3. Run `just ci` locally; CI runs the same chain. Build and run the image, then
   exercise the full flow with a mounted volume.
4. Rollback is reverting the implementation commit; the database file is
   independent of the image and can be kept or removed. No external service or
   data migration is involved.
5. After review, archive the change so the specs move into `openspec/specs/`.

## Open Questions

- Whether the project should add authentication later, and if so whether it is
  session-based or delegated to a reverse proxy.
- Whether a release workflow should additionally sign the image (for example
  cosign) or generate an SBOM beyond the build-push action defaults.
- Whether the list needs filtering (open only / done only) once the data set
  grows; not needed for the first version.
