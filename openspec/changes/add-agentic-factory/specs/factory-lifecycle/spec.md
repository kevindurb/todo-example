# Spec Delta

## Purpose

Turns one GitHub issue into one OpenSpec change and lands it through three reviewed pull requests, with visible state and explicit human gates from proposal through archive.

## ADDED Requirements

### Requirement: One issue maps to one change

The factory SHALL represent each change as a single open issue linked to exactly one OpenSpec change named `gh<issue-number>-<slug>`.

#### Scenario: Issue accepted

- **WHEN** a maintainer applies `opsx:propose` to an open issue
- **THEN** the factory targets exactly one change named `gh<issue-number>-<kebab-case-slug>`

#### Scenario: Duplicate run

- **WHEN** the issue already has an active change or an open proposal PR
- **THEN** the factory does not create a second change and reports the existing link

### Requirement: Oversized issues are split

The factory SHALL stop the propose phase when the issue requires more than one OpenSpec change and SHALL ask for the issue to be split.

#### Scenario: Multi-change issue

- **WHEN** the propose agent determines that the issue spans multiple cohesive changes
- **THEN** it reports a blocked result naming the proposed split and creates no artifacts

### Requirement: Three pull requests per change

The factory SHALL land each change through a proposal PR, an implementation PR, and an archive PR, all linked to the same issue.

#### Scenario: Full sequence

- **WHEN** all phases complete
- **THEN** the issue has one merged proposal PR, one merged implementation PR, and one merged archive PR

#### Scenario: Branch identity

- **WHEN** a phase opens its PR
- **THEN** the head branch identifies the phase and the change name

### Requirement: Phase starts require a maintainer command

The factory SHALL start a phase only when a user with write, maintain, or admin permission applies that phase's command label to an open issue.

#### Scenario: Command accepted

- **WHEN** a maintainer applies `opsx:propose`, `opsx:implement`, or `opsx:archive` to an open issue
- **THEN** the matching phase starts once and the command label is removed

#### Scenario: Non-command label

- **WHEN** a running, lifecycle, or failure label is applied to an issue
- **THEN** no phase starts

#### Scenario: Unauthorized actor

- **WHEN** a user without write, maintain, or admin permission applies a command label
- **THEN** no phase starts and the factory replies that maintainer permission is required

### Requirement: Pull requests are never merged automatically

The factory SHALL leave every pull request unmerged until a human merges it, even when all checks pass.

#### Scenario: Passing checks

- **WHEN** a factory PR has passing CI and an advisory verification comment
- **THEN** the PR remains open until a human merges it

### Requirement: The lifecycle label advances on success

The factory SHALL expose the current phase through one lifecycle label on the issue: `opsx:proposal-review`, `opsx:proposal-approved`, `opsx:impl-review`, `opsx:impl-approved`, or `opsx:archive-review`, and SHALL replace it with `opsx:done` when the archive PR merges.

#### Scenario: Proposal opens

- **WHEN** the proposal PR is opened
- **THEN** the issue carries `opsx:proposal-review`

#### Scenario: Proposal merges

- **WHEN** the proposal PR is merged
- **THEN** the issue carries `opsx:proposal-approved` and no longer carries `opsx:proposal-review`

#### Scenario: Implementation opens

- **WHEN** the implementation PR is opened
- **THEN** the issue carries `opsx:impl-review` and no longer carries `opsx:proposal-approved`

#### Scenario: Archive merges

- **WHEN** the archive PR is merged
- **THEN** the issue carries `opsx:done` and no longer carries `opsx:archive-review`

### Requirement: Running labels are additive overlays

The factory SHALL add a phase-specific running label while a phase is in progress and SHALL remove it when the phase ends, without removing the lifecycle label.

#### Scenario: Phase starts

- **WHEN** a phase starts
- **THEN** the phase-specific running label is added and the lifecycle label remains

#### Scenario: Phase succeeds

- **WHEN** the phase completes successfully
- **THEN** the running label is removed and the lifecycle label advances

#### Scenario: Phase crashes

- **WHEN** a run ends unexpectedly
- **THEN** the running label remains visible until the failure handler or a cleanup job removes it and adds `opsx:blocked`

### Requirement: Failure is visible and retryable

The factory SHALL mark a failed or blocked phase with `opsx:blocked`, post the reason and run link, and keep the lifecycle label so a maintainer can retry.

#### Scenario: Agent blocked

- **WHEN** an agent cannot complete its phase
- **THEN** the factory posts the blocking reason and adds `opsx:blocked`

#### Scenario: Retry

- **WHEN** a maintainer removes `opsx:blocked` and re-applies the command label
- **THEN** the phase runs again after re-checking its guards

### Requirement: Archive requires a merged implementation

The factory SHALL create the archive PR only when the implementation PR for the same change is merged and the change is still active.

#### Scenario: Missing implementation

- **WHEN** `opsx:archive` is applied while no implementation PR for the change is merged
- **THEN** no archive PR is created and the factory reports the missing precondition

#### Scenario: Already archived

- **WHEN** the change is already under `openspec/changes/archive/`
- **THEN** the factory reports the archive path and does not create a duplicate

### Requirement: Only the archive PR closes the issue

The proposal and implementation PRs SHALL reference the issue without a closing keyword, and the archive PR SHALL use `Closes #<issue>` so the issue closes only after archiving.

#### Scenario: Proposal and implementation

- **WHEN** the proposal or implementation PR is opened
- **THEN** its body references the issue with `Refs #<issue>` and no closing keyword

#### Scenario: Archive merge

- **WHEN** the archive PR is merged
- **THEN** GitHub closes the issue

### Requirement: Archiving is deterministic

The factory SHALL archive by moving the change under `openspec/changes/archive/` and syncing its delta specs into `openspec/specs/`, without invoking a model.

#### Scenario: Archive run

- **WHEN** the archive phase starts
- **THEN** it runs `openspec archive --yes` and validation without calling the model or joining the tailnet

#### Scenario: Validation failure

- **WHEN** validation fails after archiving
- **THEN** no archive PR is opened and the issue is marked blocked

### Requirement: The issue carries a status summary

The factory SHALL maintain one status comment on the issue with links to the three PRs, the current state, and the next human action.

#### Scenario: State change

- **WHEN** the lifecycle state changes
- **THEN** the status comment is created or updated with the current links, state, and next action

### Requirement: Runs for one issue are serialized

The factory SHALL serialize phase runs for the same issue or pull request and SHALL NOT cancel an in-flight run.

#### Scenario: Duplicate command

- **WHEN** a second command is applied while a phase for the same issue is running
- **THEN** the second run waits or exits without starting a duplicate phase

### Requirement: Verification is advisory

The factory SHALL publish verification as a comment-only review and SHALL NOT approve or request changes on the author's behalf.

#### Scenario: Verify completes

- **WHEN** the verify phase completes
- **THEN** the report is posted as a comment-only review and the implementation PR remains unapproved by the factory
