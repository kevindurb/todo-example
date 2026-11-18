# Tasks

## 1. Go module and command contract

- [x] 1.1 Replace the failing `justfile` stubs with real `fmt` (`gofmt -l` check), `lint` (`go vet ./...`), `tidy` (`go mod tidy`), `build`, `test`, `run`, `container-build`, and `container-run` recipes, and change `ci` to `pre-commit pre-push spec-validate container-build`; verify `just --list` shows every recipe and `just fmt` exits zero.
- [x] 1.2 Initialize the Go module `github.com/kevindurb/todo-example` with a Go 1.24 language version, add `modernc.org/sqlite`, add a minimal compilable `cmd/todo` entry point, and verify `just tidy` and `just build` both succeed.
- [x] 1.3 Add local build and data paths to `.gitignore` (`bin/`, `data/`, `*.db`, `*.db-wal`, `*.db-shm`) and verify `just build` leaves no tracked files under those paths.

## 2. Todo model and storage

- [x] 2.1 Implement the `internal/todo` record, field limits, trimming, completion state, and validation results; verify unit tests cover a valid todo, a blank or whitespace title, an oversized title, and oversized notes.
- [x] 2.2 Implement `internal/store` opening, parent-directory creation, `PRAGMA user_version` schema initialization, WAL, busy timeout, and a clear startup error for an unusable path; verify tests cover first start, repeated start, a custom path, and a read-only or invalid path.
- [x] 2.3 Implement store create, get, update, set-status, delete, and ordered list operations with bound parameters and a distinct not-found error; verify tests cover CRUD, open-before-done ordering, newest-first ordering, SQL metacharacters stored literally, and concurrent creates all persisting.

## 3. Server-rendered web interface

- [x] 3.1 Implement the routes and handlers for the list, new form, create, edit form, update, completion state, delete confirmation, and delete, using `303` redirects after successful mutations and explicit `404`/`405` responses; verify `httptest` tests cover each route, the redirects, unknown identifiers, and unsupported methods.
- [x] 3.2 Add the embedded templates and local stylesheet for the list, form, confirmation, and error pages, with labelled inputs, an empty state, and no client-side script; verify a test asserts every page renders, a title containing HTML markup is escaped, and no rendered page contains a script element or a third-party asset URL.
- [x] 3.3 Render validation failures as a form re-display with a client-error status and the submitted values preserved, cap request bodies with `MaxBytesReader`, and add conservative response headers; verify tests cover a failed create, a failed edit, an oversized body, and the headers.
- [x] 3.4 Ensure the delete flow shows a confirmation page and removes the todo only when the confirmation form is submitted; verify a test loads the confirmation page without deleting and then submits it and observes the delete.

## 4. Process lifecycle and operations

- [x] 4.1 Implement `cmd/todo` to load `ADDR` and `DATABASE_PATH` with documented defaults, open the store, mount the web handler, and serve HTTP; verify `just test` and `just build` pass and a test starts the handler against a temporary database.
- [x] 4.2 Add `GET /healthz` that pings the database with a short timeout and returns success only when it is reachable; verify handler tests cover the healthy and unavailable cases.
- [x] 4.3 Handle `SIGINT` and `SIGTERM` with a bounded graceful shutdown that stops accepting requests, drains in-flight work, and closes the store; verify a test exercises the shutdown path and the process exits cleanly.
- [x] 4.4 Add structured request logging that records method, path, status, and duration without logging form values; verify a test captures a request log line and asserts that the todo title and notes do not appear in it.

## 5. Container and project documentation

- [x] 5.1 Add a multi-stage `Dockerfile` that builds with `CGO_ENABLED=0` and runs the binary as a non-root user from a minimal runtime image, plus a `.dockerignore`; verify `just container-build` succeeds and the runtime stage contains no Go toolchain or source tree.
- [x] 5.2 Set the container defaults (`ADDR=:8080`, `DATABASE_PATH=/data/todos.db`), create the writable data directory with the runtime user's ownership, and document the volume mount; verify a container started with a named volume can create the database and a second start against the same volume still lists the todos.
- [x] 5.3 Replace the template text in `README.md` with the app's purpose, quick start, `just` commands, configuration, and deployment pointer; verify every documented command exists in `just --list` and the quick start works in a clean checkout.
- [x] 5.4 Add `docs/deployment.md` covering image build and publish, the GHCR image name, `docker run` with a volume, the health check, the no-authentication warning, and backup of the SQLite file; verify its commands match the `Dockerfile`, `justfile`, and workflow defaults.
- [x] 5.5 Update `AGENTS.md` with the project purpose, the real commands, the Go/SQLite conventions, and the single-writer constraint; verify the commands table matches `just --list` and no stale template claims remain.
- [x] 5.6 Update the `context:` block in `openspec/config.yaml` with the Go/SQLite stack, the real `just` commands, and the app's boundaries; verify it names only commands that exist and `openspec validate --all` still passes.

## 6. Integration checks

- [x] 6.1 Run `just ci` and `openspec validate gh1-basic-todo-app --strict`; verify both pass, and confirm the CI workflow's existing `just ci` step now exercises formatting, vetting, building, testing, the container build, and spec validation.

## Maintainer prerequisite (control plane)

These are not implementation-agent tasks. The apply agent must not edit
`.github/`; a maintainer adds and reviews this workflow directly.

- [x] Add `.github/workflows/container.yml` with a pull-request build and a
  `main`/version-tag publish to GHCR. Use `permissions: contents: read` and
  `packages: write`, `docker/login-action` with `GITHUB_TOKEN`,
  `docker/metadata-action`, and `docker/build-push-action` pinned to immutable
  SHAs, with provenance and SBOM generation enabled. Verify with `actionlint`
  and one pull-request build plus one `main` publish; confirm the published
  image is pullable and tagged with the commit SHA and a moving tag.
  *Satisfied on `main` before implementation: the pull-request build passes,
  and the workflow publishes `latest`, `sha-<commit>`, and `v<version>` tags.*
- [x] Confirm the package visibility and branch protection still allow the
  intended reviewers and the `CI` check.

## Workflow follow-up

- [x] Before applying `opsx:archive`, confirm the container workflow is on
  `main` and the published image can be pulled with the documented command.
  *The workflow is on `main` and the pull-request build passes; the documented
  tags now match what it publishes.*
- Archive the change after review, then verify the synced specs under
  `openspec/specs/` and the archived change path.
