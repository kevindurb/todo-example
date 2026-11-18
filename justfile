set shell := ["bash", "-euc"]

default:
    @just --list

# --- Contract -------------------------------------------------------------
# Hooks and CI call these. `fmt` and `lint` are checks: exit non-zero when
# something is wrong, do not fix it in place.

fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -l .)"
    if [ -n "$unformatted" ]; then
        echo "gofmt found unformatted files:" >&2
        echo "$unformatted" >&2
        exit 1
    fi

lint:
    go vet ./...

tidy:
    go mod tidy

build:
    go build -trimpath -o bin/todo ./cmd/todo

test:
    go test ./...

run:
    go run ./cmd/todo

container-build:
    docker build --tag todo-example:local .

container-run:
    docker run --rm --publish 8080:8080 --volume todo-data:/data todo-example:local

# Opt-in acceptance test; pass a container runtime (and flags) as the argument.
container-test runtime="docker":
    TODO_CONTAINER_TEST=1 TODO_CONTAINER="{{runtime}}" go test -count=1 -tags container ./cmd/todo

pre-commit: fmt lint
pre-push: build test

ci: pre-commit pre-push spec-validate container-build

install-hooks:
    git config core.hooksPath .githooks
    @echo "Hooks installed: pre-commit and pre-push now run via 'just'."

spec-list:
    openspec list

spec-validate:
    openspec validate --all
