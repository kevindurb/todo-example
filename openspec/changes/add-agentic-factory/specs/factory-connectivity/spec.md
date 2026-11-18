# Spec Delta

## Purpose

Defines how the factory reaches the private Aperture model and MCP endpoints through an ephemeral Tailscale node joined with GitHub WIF, and the constraints that keep that access reproducible and least-privileged.

## ADDED Requirements

### Requirement: Ephemeral tailnet membership

The factory SHALL join the tailnet as an ephemeral node tagged `tag:ci` before any model or MCP call and SHALL leave the tailnet when the job ends.

#### Scenario: Connection

- **WHEN** a phase that needs the model or MCP starts
- **THEN** the workflow exchanges a GitHub OIDC token through a Tailscale federated identity, joins as `tag:ci`, and the node is ephemeral

#### Scenario: Short-lived credential

- **WHEN** the workflow requests the tailnet connection
- **THEN** it uses a short-lived token derived from the OIDC exchange and stores no long-lived Tailscale credential

#### Scenario: Model-free phase

- **WHEN** the archive phase runs
- **THEN** it does not join the tailnet

### Requirement: Private model endpoint

The factory SHALL use the Aperture OpenAI-compatible endpoint at `https://ai.beaver-cloud.ts.net/v1` with the `deepseek-v4.1-flash` model and SHALL declare the model's real context and output limits.

#### Scenario: Model selection

- **WHEN** an agent session starts
- **THEN** the default model is `aperture/deepseek-v4.1-flash` and the provider endpoint is the private host

#### Scenario: Limits

- **WHEN** the model is configured
- **THEN** its 1,048,576-token context and 384,000-token output limits are declared

#### Scenario: HTTPS

- **WHEN** the endpoint is configured
- **THEN** it uses `https://`

### Requirement: Private MCP endpoint

The factory SHALL use the MCP endpoint at `https://ai.beaver-cloud.ts.net/v1/mcp` in remote Streamable HTTP mode without OAuth and SHALL expose its tools natively rather than through Code Mode.

#### Scenario: Connection

- **WHEN** an agent allowed MCP tools starts
- **THEN** it connects to the private endpoint and does not attempt OAuth

#### Scenario: Native tools

- **WHEN** the MCP server is configured
- **THEN** Code Mode is disabled so its tools appear directly to the agent

#### Scenario: HTTPS

- **WHEN** the MCP endpoint is configured
- **THEN** it uses `https://`

### Requirement: Preflight before the agent

The factory SHALL verify model reachability before starting an agent and SHALL stop the phase with a clear connectivity error when the endpoint is unreachable.

#### Scenario: Unreachable

- **WHEN** the model endpoint does not respond after Tailscale connects
- **THEN** the phase stops before the agent starts and reports the failure

#### Scenario: Slow propagation

- **WHEN** the tailnet peer is not immediately reachable
- **THEN** the preflight retries for a bounded period

### Requirement: WIF setup is verifiable before a phase

The factory SHALL provide a dispatchable smoke workflow that joins the tailnet and checks the model endpoint without invoking the model.

#### Scenario: Smoke run

- **WHEN** a maintainer dispatches the smoke workflow
- **THEN** it exchanges the GitHub OIDC token, joins as `tag:ci`, makes a one-token completion with the configured model, and stops before the agent starts

#### Scenario: Claims visible

- **WHEN** the smoke workflow runs
- **THEN** it prints the non-secret `iss`, `aud`, `sub`, `repository`, `ref`, `event_name`, and `workflow` claims without printing the token

#### Scenario: Immutable subject

- **WHEN** GitHub issues an immutable OIDC subject containing the owner and repository IDs
- **THEN** the federated identity's subject matches that exact prefix and the smoke exchange succeeds

### Requirement: Credentials are isolated from the agent

The factory SHALL authenticate to Tailscale with a short-lived GitHub OIDC token exchanged through a federated identity and SHALL NOT store a long-lived Tailscale credential. It SHALL NOT expose the OIDC request variables or the federated identity configuration to the agent step.

#### Scenario: Agent environment

- **WHEN** the agent runs
- **THEN** `ACTIONS_ID_TOKEN_REQUEST_URL`, `ACTIONS_ID_TOKEN_REQUEST_TOKEN`, `TS_CLIENT_ID`, and `TS_AUDIENCE` are not present in its environment

#### Scenario: No long-lived secret

- **WHEN** the repository is configured
- **THEN** no `TS_OAUTH_SECRET` and no long-lived Tailscale auth key are stored

#### Scenario: Missing permission

- **WHEN** the workflow does not grant `id-token: write`
- **THEN** the phase stops before the agent starts and reports the connectivity failure

#### Scenario: Role required for a tag

- **WHEN** the tailnet policy gives `tag:ci` model access but no base role
- **THEN** `/api/wif/claims` and the model endpoint return 403 `access denied: no role granted` and the preflight stops before the agent starts

#### Scenario: Role and model grant

- **WHEN** the tailnet policy's `tag:ci` app capability includes `role: user` and model access
- **THEN** the preflight completion succeeds and no model API key is stored or passed

### Requirement: CI access is least-privileged

The factory's CI tag SHALL be allowed to reach only the model and MCP host on the HTTPS port and SHALL NOT reach other tailnet services or SSH.

#### Scenario: Allowed path

- **WHEN** the CI node connects to the tailnet
- **THEN** its ACL permits only `tag:ci` to the AI host on port 443

#### Scenario: Denied paths

- **WHEN** the CI node attempts another tailnet host or port
- **THEN** the connection is denied

#### Scenario: Peer visibility

- **WHEN** the tailnet ACL lacks a grant from `tag:ci` to the AI host
- **THEN** Tailscale's netmap trimming omits the host from the runner's `tailscale status` and the preflight fails before the model call
