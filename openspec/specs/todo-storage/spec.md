# todo-storage Specification

## Purpose
Defines durable todo storage: a self-contained SQLite database that survives restarts, is initialized automatically, and is configured by path.

## Requirements

### Requirement: Todos persist across process restarts

The system SHALL store todos in a local SQLite database so that committed records remain available after the process stops and starts again.

#### Scenario: Restart preserves data

- **WHEN** a todo is created and the process is restarted against the same database file
- **THEN** the todo is still listed with the same identifier, fields, and timestamps

#### Scenario: No external database

- **WHEN** the application starts with a writable database path
- **THEN** it needs no database server or network service to read and write todos

### Requirement: The schema is initialized automatically

On startup the system SHALL create the todo table and any required index when they are absent and SHALL record the applied schema version so repeated starts are safe.

#### Scenario: First start

- **WHEN** the configured database file does not exist
- **THEN** the system creates the file and the schema before serving requests

#### Scenario: Repeated start

- **WHEN** the process starts against a database that already has the current schema
- **THEN** it does not destroy or duplicate existing todos

### Requirement: Database access is parameterized

The system SHALL pass every user-supplied value to SQLite as a bound parameter and SHALL NOT construct SQL by concatenating user input.

#### Scenario: SQL metacharacters in a title

- **WHEN** a todo title contains quotes, semicolons, or SQL keywords
- **THEN** the title is stored and returned literally and no other row is changed or removed

#### Scenario: Unknown identifier

- **WHEN** a lookup uses an identifier with no matching row
- **THEN** the store reports not-found rather than returning a zero-valued todo as if it existed

### Requirement: The database path is configurable

The system SHALL read the database location from configuration and SHALL create missing parent directories for that path before opening the database.

#### Scenario: Custom path

- **WHEN** the database path is set to a writable location
- **THEN** the system creates and uses that file instead of a default path

#### Scenario: Unusable path

- **WHEN** the configured path cannot be created or opened
- **THEN** startup fails with a clear error naming the path and the process exits non-zero

### Requirement: Concurrent requests do not lose committed writes

The system SHALL serialize database writes and SHALL apply each accepted change exactly once, even when requests arrive concurrently.

#### Scenario: Concurrent creates

- **WHEN** several create requests are accepted at the same time
- **THEN** every accepted todo is present after the requests complete

#### Scenario: Busy database

- **WHEN** a write is blocked by another write
- **THEN** the system waits for a bounded period and either completes the write or returns an explicit error instead of silently dropping it
