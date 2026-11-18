# Spec Delta

## Purpose

Defines the deployable shape of the application: a small non-root container image, environment configuration, persistent data, health checking, graceful shutdown, and the GitHub Actions build and publish path.

## ADDED Requirements

### Requirement: The application ships as a container image

The repository SHALL provide a multi-stage container build that compiles the application and produces a runtime image containing the binary and presentation assets but no compiler or build toolchain.

#### Scenario: Image build

- **WHEN** a user builds the container from the repository root
- **THEN** the build succeeds without requiring a Go toolchain on the host and produces an image that starts the application

#### Scenario: Runtime contents

- **WHEN** the runtime image is inspected
- **THEN** it contains the application binary and its templates and stylesheet but not the Go compiler or source tree

### Requirement: The container runs as a non-root user

The runtime image SHALL use an unprivileged user and SHALL require write access only to the configured data location.

#### Scenario: Default start

- **WHEN** the image is started without overrides
- **THEN** the process runs as a user other than root and serves the application

#### Scenario: Data location writable

- **WHEN** a writable volume is mounted at the configured data directory
- **THEN** the application can create and update its database there

### Requirement: Runtime behavior is configured by environment

The system SHALL read its listen address and database path from `ADDR` and `DATABASE_PATH`, SHALL apply documented defaults when they are unset, and SHALL fail fast with a clear message when a value is unusable.

#### Scenario: Defaults

- **WHEN** neither variable is set
- **THEN** the application listens on its documented default address and uses its documented default database path

#### Scenario: Invalid address

- **WHEN** `ADDR` is not a valid listen address
- **THEN** startup fails with a message naming the value and the process exits non-zero

### Requirement: A health endpoint reports readiness

The application SHALL serve `GET /healthz` without authentication, returning success only when the database is reachable and service is ready, and a failure status otherwise.

#### Scenario: Healthy

- **WHEN** the database is open and reachable
- **THEN** `GET /healthz` returns a success status

#### Scenario: Database unavailable

- **WHEN** the database cannot be reached
- **THEN** `GET /healthz` returns a failure status rather than success

### Requirement: Shutdown is graceful

On a termination signal the application SHALL stop accepting new requests, allow in-flight requests a bounded time to finish, close the database, and exit.

#### Scenario: Termination signal

- **WHEN** the process receives a termination signal while serving
- **THEN** it stops accepting connections, completes or times out in-flight requests, closes the database, and exits cleanly

#### Scenario: Repeated signal

- **WHEN** a second termination signal arrives during shutdown
- **THEN** the process exits without corrupting the database

### Requirement: GitHub Actions builds and publishes the image

The repository SHALL build the image in GitHub Actions and SHALL publish it to the GitHub Container Registry when changes land on the default branch or a release tag, tagging the image with the commit and a moving default tag.

#### Scenario: Pull request build

- **WHEN** a pull request changes the application or container files
- **THEN** the workflow builds the image without publishing it

#### Scenario: Default branch publish

- **WHEN** a commit lands on the default branch
- **THEN** the workflow publishes an image tagged with the commit identifier and a moving default tag

#### Scenario: Release tag publish

- **WHEN** a version tag is pushed
- **THEN** the workflow publishes an image tagged with that version

### Requirement: Published images contain no credentials

The image build SHALL NOT embed repository secrets, registry credentials, or runtime configuration that belongs in the environment.

#### Scenario: Inspect layers

- **WHEN** the published image is inspected
- **THEN** it contains no registry token, GitHub token, or database credentials
