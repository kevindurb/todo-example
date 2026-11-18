# todo-management Specification

## Purpose
Defines the todo record and the user-visible lifecycle of creating, editing, completing, and deleting a todo, including validation and not-found behavior.

## Requirements

### Requirement: A todo has a stable identity and basic fields

Every todo SHALL have a server-assigned numeric identifier, a required title, optional notes, a completion state, and creation and update timestamps. The identifier SHALL remain stable for the life of the record.

#### Scenario: Record shape

- **WHEN** a todo is created
- **THEN** it is returned with a numeric identifier, its title and notes, a completion state of not done, and creation and update timestamps

#### Scenario: Identifier stability

- **WHEN** an existing todo is edited or its completion state changes
- **THEN** its identifier and creation timestamp stay the same and only its update timestamp changes

### Requirement: A user can create a todo

The system SHALL create a todo from a valid title and optional notes and SHALL return the user to the list with the new todo visible.

#### Scenario: Successful creation

- **WHEN** a user submits a non-blank title of at most 200 characters and notes of at most 2,000 characters
- **THEN** a new not-done todo is stored and the list shows it

#### Scenario: Notes are optional

- **WHEN** a user submits a valid title with empty notes
- **THEN** the todo is created with empty notes

### Requirement: Invalid todo input is rejected

The system SHALL reject a blank or whitespace-only title, a title longer than 200 characters, and notes longer than 2,000 characters, and SHALL NOT create or update a record for a rejected request.

#### Scenario: Blank title

- **WHEN** a user submits a title that is empty or only whitespace
- **THEN** the request is rejected with a validation message and no todo is created

#### Scenario: Oversized field

- **WHEN** a user submits a title over 200 characters or notes over 2,000 characters
- **THEN** the request is rejected with a validation message and no todo is created or updated

### Requirement: A user can edit a todo

The system SHALL let a user change the title and notes of an existing todo while preserving its identifier, creation timestamp, and completion state.

#### Scenario: Successful edit

- **WHEN** a user submits valid replacement title and notes for an existing todo
- **THEN** the stored title and notes change, the update timestamp advances, and the completion state is unchanged

#### Scenario: Edit a missing todo

- **WHEN** a user submits an edit for an identifier that does not exist
- **THEN** the system reports that the todo was not found and creates no record

### Requirement: A user can change a todo's completion state

The system SHALL let a user set an existing todo to done or not done, and repeating the same requested state SHALL leave the record in that state.

#### Scenario: Mark done

- **WHEN** a user marks a not-done todo as done
- **THEN** the todo is stored as done and the list shows it as completed

#### Scenario: Mark not done

- **WHEN** a user marks a done todo as not done
- **THEN** the todo is stored as not done and the list shows it as open

#### Scenario: Repeated request

- **WHEN** a user marks an already-done todo as done again
- **THEN** the todo remains done and no duplicate record is created

### Requirement: A user can delete a todo

The system SHALL permanently remove an existing todo when deletion is confirmed and SHALL report a missing todo without changing anything when it does not exist.

#### Scenario: Confirm deletion

- **WHEN** a user confirms deletion of an existing todo
- **THEN** the todo is removed from storage and no longer appears in the list

#### Scenario: Delete a missing todo

- **WHEN** a user confirms deletion of an identifier that does not exist
- **THEN** the system reports that the todo was not found and changes nothing

### Requirement: The list presents open todos before completed todos

The system SHALL list open todos before done todos, and within each group SHALL list the most recently created todo first.

#### Scenario: Mixed list

- **WHEN** the list contains both open and done todos
- **THEN** every open todo appears before every done todo

#### Scenario: Ordering within a group

- **WHEN** several open todos exist
- **THEN** the newest of them appears first and the oldest appears last
