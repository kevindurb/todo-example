# Deployment

The app ships as one container image: a static Go binary on a minimal runtime,
running as a non-root user, with SQLite on a mounted data volume.

> **No authentication.** Every request is treated as the same single user. Run
> the app on a private network or behind an authenticating proxy. Do not publish
> it directly to the internet.

## Image

| | |
|---|---|
| Registry | GitHub Container Registry |
| Image | `ghcr.io/kevindurb/todo-example` |
| Default tag | `main` (moving) |
| Commit tag | `sha-<commit>` |
| Release tag | the pushed version tag, for example `1.2.3` |
| Port | `8080` |
| Data path | `/data/todos.db` |
| Runtime user | non-root (`65532`) |

Building the image locally:

```sh
just container-build          # tags todo-example:local
just container-run            # runs it with a todo-data volume
```

`just container-build` runs the multi-stage build in the `Dockerfile`: a
`golang:1.24-alpine` stage compiles with `CGO_ENABLED=0`, then the binary is
copied to `gcr.io/distroless/static-debian12:nonroot`. The runtime image has no
shell, package manager, compiler, or source tree.

`just container-test` is an opt-in acceptance test. It builds the image, checks
that the runtime is non-root and contains no Go toolchain or source, then starts
the container twice against one named volume to confirm a created todo survives
the restart. It needs a container runtime and network access for the base
images; pass a runtime with flags if it is not plain `docker`, for example
`just container-test "podman --root /tmp/podman --runroot /tmp/podman-run"`.

## Run it

```sh
docker run --detach \
  --name todo \
  --publish 8080:8080 \
  --volume todo-data:/data \
  ghcr.io/kevindurb/todo-example:main
```

Then open <http://localhost:8080>. The named volume keeps the database across
container replacements. The app creates `/data/todos.db` and its schema on first
start; a second start against the same volume lists the existing todos.

To override the defaults:

```sh
docker run --detach \
  --name todo \
  --publish 9000:9000 \
  --env ADDR=:9000 \
  --env DATABASE_PATH=/data/todos.db \
  --volume todo-data:/data \
  ghcr.io/kevindurb/todo-example:main
```

| Variable | Container default | Meaning |
|---|---|---|
| `ADDR` | `:8080` | Listen address in `host:port` form |
| `DATABASE_PATH` | `/data/todos.db` | SQLite file inside the writable volume |

Startup fails with a message naming a bad `ADDR` or an unusable
`DATABASE_PATH`.

## Health check

`GET /healthz` pings the database and returns `200` with `ok` only when it is
reachable, otherwise `503`:

```sh
curl -fsS http://localhost:8080/healthz
```

Point your orchestrator or reverse proxy readiness probe at it. There is no
`HEALTHCHECK` in the image; liveness and restart policy belong to the platform.

## Publishing

Publishing is a GitHub Actions job (`.github/workflows/container.yml`), not a
local command. The workflow builds the image on pull requests without
publishing, and publishes to GHCR when a commit lands on `main` or a version tag
is pushed. It authenticates with the workflow's `GITHUB_TOKEN` and
`packages: write`; no registry secret is stored in the repository.

The workflow is repository infrastructure and is maintained separately from the
application code. If the image is not published yet, build and push it from the
repository root with the `Dockerfile` and the tag convention above.

## Backup and restore

SQLite writes in WAL mode. For a consistent copy, stop the container first and
then copy the database:

```sh
docker stop todo
docker cp todo:/data/todos.db ./todos.db
docker start todo
```

If you copy while the app is running, copy `todos.db`, `todos.db-wal`, and
`todos.db-shm` together and keep them in the same directory.

Restore by stopping the container, replacing `/data/todos.db` (and removing any
stale `-wal`/`-shm` files), and starting it again:

```sh
docker stop todo
docker cp ./todos.db todo:/data/todos.db
docker start todo
```

Keep exactly one app process per database file: SQLite is a single-writer store
and the app assumes one container per volume.
