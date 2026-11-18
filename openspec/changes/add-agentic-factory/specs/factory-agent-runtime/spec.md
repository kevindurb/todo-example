# Spec Delta

## Purpose

Defines how unattended phase agents receive context, what they may change or execute, and how they report success, blockage, or failure without ever performing GitHub side effects.

## ADDED Requirements

### Requirement: Static prompts and file-based context

The factory SHALL pass each agent a static prompt and SHALL supply event-specific data only through `.agent/context.md`.

#### Scenario: Prompt contains no dynamic text

- **WHEN** a phase starts
- **THEN** the prompt does not embed issue, PR, or review text and points the agent at the context file

#### Scenario: Untrusted content is delimited

- **WHEN** the context includes issue, PR, or review text
- **THEN** that text is marked as untrusted data and the agent's system prompt states that repository instructions take precedence

### Requirement: Agents are explicitly selected by phase

The factory SHALL run each phase with the named agent for that phase and SHALL NOT allow the model to choose a different phase agent or launch a subagent.

#### Scenario: Phase selection

- **WHEN** a workflow starts a phase
- **THEN** it selects that phase's agent explicitly

#### Scenario: Subagent denied

- **WHEN** an agent attempts to launch a subagent
- **THEN** the subagent permission is denied

### Requirement: Agents perform no GitHub side effects

The agent SHALL NOT push, merge, label, comment, or open pull requests; the workflow SHALL perform all GitHub side effects.

#### Scenario: No credentials

- **WHEN** the agent runs
- **THEN** it has no GitHub token, no GitHub OIDC request variables, no `gh` command, and no persisted checkout credential

#### Scenario: Attempted push

- **WHEN** the agent attempts `git push`
- **THEN** the permission is denied and the phase reports failure

### Requirement: Result contract

Each agent SHALL write `.agent/result.json` with `status`, `phase`, `change`, and `summary`, plus `blocked_reason` or `verdict` where applicable, and SHALL write `.agent/report.md` for the human-facing report.

#### Scenario: Success

- **WHEN** the phase completes
- **THEN** `status` is `ok` and the report is usable as the PR or comment body

#### Scenario: Missing result

- **WHEN** the result file is missing or invalid
- **THEN** the factory treats the phase as failed, uses a fallback report, and does not open or update a PR

#### Scenario: Blocked

- **WHEN** the agent cannot proceed
- **THEN** `status` is `blocked` and `blocked_reason` names the missing input or decision

### Requirement: Write scopes match the phase

Each phase SHALL have a write scope limited to what it needs, and the factory control plane SHALL be read-only to every agent.

#### Scenario: Propose scope

- **WHEN** the propose agent runs
- **THEN** it may write only under the change directory and `.agent/`

#### Scenario: Apply scope

- **WHEN** the apply agent runs
- **THEN** it may write project code and the active change but not `.github/`, `.opencode/`, `.agents/`, `.githooks/`, or `.git/`

#### Scenario: Verify scope

- **WHEN** the verify agent runs
- **THEN** it may write only under `.agent/` and may not edit project code

#### Scenario: Revise scope

- **WHEN** the revise agent runs on a proposal PR
- **THEN** its writes are limited to the change artifacts; on an implementation PR it may edit code but not the control plane

### Requirement: Control-plane changes are blocked for humans

The agent SHALL NOT modify workflow files, action definitions, OpenCode configuration, repository skills, or hooks; a change requiring those SHALL be reported as blocked for a human.

#### Scenario: Needs a control-plane change

- **WHEN** the implementation requires a change under `.github/` or `.opencode/`
- **THEN** the agent reports `blocked` and does not edit the file

### Requirement: Shell access is limited to the phase

The factory SHALL restrict shell commands to the phase's needs and SHALL deny raw network clients that could bypass MCP permissions.

#### Scenario: Propose shell

- **WHEN** the propose agent runs
- **THEN** it may run `openspec` and read-only git commands but not `curl`, `wget`, `ssh`, `gh`, or `git push`

#### Scenario: Apply and revise shell

- **WHEN** the apply or revise agent runs
- **THEN** it may run `just` and `openspec` commands but not raw network clients or GitHub commands

#### Scenario: Verify shell

- **WHEN** the verify agent runs
- **THEN** it has no shell access

### Requirement: MCP access is limited to read-only web tools

The factory SHALL expose only the MCP tools a phase needs and SHALL deny tools with side effects on the tailnet or browser.

#### Scenario: Propose and verify

- **WHEN** the propose or verify agent runs
- **THEN** it may use the read-only web search and fetch tools

#### Scenario: Dangerous tools

- **WHEN** any factory agent runs
- **THEN** `TailnetSSH_*`, `chrome_*`, and session-inspection tools are denied

#### Scenario: Apply and revise

- **WHEN** the apply or revise agent runs
- **THEN** no MCP tools are available

### Requirement: Runs are bounded

The factory SHALL bound each agent by a maximum number of model steps and the workflow by a timeout.

#### Scenario: Step limit

- **WHEN** an agent reaches its step limit
- **THEN** it stops and summarizes instead of continuing

#### Scenario: Timeout

- **WHEN** a run exceeds the workflow timeout
- **THEN** the job ends and the failure handler marks the phase blocked

### Requirement: Only agent phases use the model

The factory SHALL invoke the model only for propose, apply, verify, and revise; deterministic phases SHALL NOT use the model.

#### Scenario: Archive

- **WHEN** the archive phase runs
- **THEN** no model call is made
