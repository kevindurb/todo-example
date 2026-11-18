# todo-web-ui Specification

## Purpose
Defines the server-rendered web interface: the pages and form flow a person uses to manage todos, with no client-side JavaScript.

## Requirements

### Requirement: The interface works without client-side JavaScript

Every page SHALL be usable with JavaScript disabled, and the application SHALL NOT emit script elements or serve JavaScript assets.

#### Scenario: JavaScript disabled

- **WHEN** a browser loads any page with JavaScript disabled
- **THEN** listing, creating, editing, completing, and deleting remain available through links and forms

#### Scenario: No script content

- **WHEN** any page is rendered
- **THEN** its HTML contains no script element and references no JavaScript file

### Requirement: The list page shows todos and their available actions

The list page SHALL show each todo's title, its notes when present, and whether it is done, and SHALL offer controls to add, edit, complete or reopen, and delete a todo.

#### Scenario: Populated list

- **WHEN** at least one todo exists
- **THEN** the page shows each todo's title and completion state with controls for editing, changing completion, and deleting it

#### Scenario: Empty list

- **WHEN** no todo exists
- **THEN** the page states that there are no todos and still offers the add form

### Requirement: Forms are labelled and preserve submitted values

The create and edit forms SHALL give every input an associated label, SHALL mark the title as required, and SHALL re-display the submitted title and notes when validation fails.

#### Scenario: Labelled controls

- **WHEN** a create or edit form is rendered
- **THEN** the title and notes controls each have an associated label

#### Scenario: Preserved input

- **WHEN** a submission fails validation
- **THEN** the form is re-rendered with the submitted title and notes still present

### Requirement: Successful mutations redirect to the list

After a successful create, edit, completion change, or deletion, the system SHALL respond with a redirect to the list page rather than rendering a result page directly.

#### Scenario: Create redirect

- **WHEN** a todo is created successfully
- **THEN** the response is a redirect to the list and a later load shows the new todo

#### Scenario: Delete redirect

- **WHEN** a todo is deleted successfully
- **THEN** the response is a redirect to the list and the todo is absent

### Requirement: Validation failures re-render the form

When a create or edit submission fails validation, the system SHALL respond with a client-error status, show the applicable messages, and keep the user on the form.

#### Scenario: Invalid create

- **WHEN** a create submission has a blank title
- **THEN** the form is shown again with a validation message and a client-error status

#### Scenario: Invalid edit

- **WHEN** an edit submission has notes over the limit
- **THEN** the form is shown again with a validation message and the original todo is unchanged

### Requirement: Deletion requires confirmation

The system SHALL show a confirmation page for a delete request and SHALL delete only when the confirmation form is submitted.

#### Scenario: Confirmation page

- **WHEN** a user selects delete for an existing todo
- **THEN** a page names the todo and offers a confirm control and a cancel link

#### Scenario: Not deleted on view

- **WHEN** the confirmation page is loaded but not submitted
- **THEN** the todo remains in storage

### Requirement: User content is escaped in HTML

The system SHALL HTML-escape todo titles, notes, and validation messages wherever they appear in a page.

#### Scenario: Markup in a title

- **WHEN** a todo title contains HTML markup characters
- **THEN** the page shows those characters as text and does not render them as elements

### Requirement: Unknown routes and unsupported methods are explicit

The system SHALL return not-found for an unknown path or todo, and method-not-allowed with an `Allow` header for a known path used with an unsupported method.

#### Scenario: Unknown todo

- **WHEN** a request names a todo identifier that does not exist
- **THEN** the response is not-found and no record is created or changed

#### Scenario: Wrong method

- **WHEN** a request uses an unsupported method on a known route
- **THEN** the response is method-not-allowed and names the allowed methods

### Requirement: The interface uses only local assets

The system SHALL serve its stylesheet and any other presentation asset from the application itself and SHALL NOT depend on a third-party script, stylesheet, or font host.

#### Scenario: Styling available offline

- **WHEN** the application is loaded on a network that cannot reach third-party hosts
- **THEN** its styling still applies and no third-party asset request is made
