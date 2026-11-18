# Proposal

## Why

`base` already carries a strong local workflow — OpenSpec artifacts, a `just`
command contract, git hooks, and agent instructions — but every phase still
depends on a human driving the agent, moving branches, opening PRs, and
archiving the change. That makes the workflow hard to apply consistently and
impossible to run unattended.

The factory makes GitHub the control plane: an issue starts a change, a
proposal PR is the spec gate, an implementation PR is the code gate, and an
archive PR closes the loop. Humans keep the judgment — review and merge — while
the mechanical work becomes repeatable and auditable.

This is possible now because OpenSpec 2.0 and OpenCode V2 exist, the factory's
own composite action can pin a checksum-verified OpenCode binary, and the
Aperture host on the tailnet provides a model and MCP endpoint without per-run
API keys. The template should carry this as the default for new repositories
instead of each project rebuilding it.

## What Changes

- Add a GitHub-native factory lifecycle: one issue maps to one OpenSpec change
  named `gh<issue>-<slug>` and to three PRs — proposal, implementation, and
  archive — all linked back to the same issue.
- Add human gates as explicit GitHub actions: merge the proposal PR (spec
  approval), apply `opsx:implement` (build), merge the implementation PR (code
  approval), apply `opsx:archive` (archive), merge the archive PR (done).
- Add command labels, running overlays, lifecycle labels, and a failure
  overlay. Running labels are additive so a crashed run leaves the lifecycle
  state visible.
- Run phase-specific agents unattended: a static prompt, `.agent/context.md`
  as the only dynamic input, `.agent/result.json` as the machine-readable
  outcome, and write scopes that exclude the factory's own control plane.
- Keep the agent out of GitHub. Workflows — not the model — commit, push, open
  PRs, label issues, and comment. Checkout does not persist credentials.
- Make archiving deterministic and model-free: `openspec archive --yes`,
  validation, and an archive PR whose body says `Closes #N`. Only that PR
  closes the issue.
- Add an advisory verify agent on implementation PRs and a revise path that
  feeds review comments back to the appropriate phase agent.
- Connect CI to the private Aperture endpoint through an ephemeral Tailscale
  node joined with a GitHub OIDC token exchanged through a federated identity:
  `deepseek-v4.1-flash` at `https://ai.beaver-cloud.ts.net/v1`, MCP at
  `/v1/mcp`, with dangerous MCP tools denied.
- Add the supporting contracts: issue template, label setup, status comment,
  failure/retry behavior, docs, and a durable `.agents/skills` convention.
- Update `AGENTS.md` and `README.md` so the factory is the documented default
  feature workflow for repos created from this template.

**Not in v1:** auto-merge, autonomous issue triage, a quick-fix lane that
bypasses OpenSpec, multi-repo orchestration, and a GitHub App for bot-driven
label transitions.

## Capabilities

### New Capabilities

- `factory-lifecycle`: the issue-centric state machine. Covers one issue per
  change, the three-PR sequence, command/running/lifecycle labels, transition
  guards, `Refs` versus `Closes`, archive semantics, status reporting, and
  failure/retry behavior.
- `factory-agent-runtime`: unattended phase execution. Covers the context and
  result contract, phase-specific write scopes, shell and MCP restrictions,
  step and timeout limits, blocked/failed reporting, and the rule that the
  agent performs no GitHub side effects.
- `factory-connectivity`: private model and MCP access. Covers the ephemeral
  Tailscale node, the Aperture OpenAI-compatible provider and MCP server
  configuration, preflight checks, and the security constraints around the
  tailnet ACL, credentials, and dangerous MCP tools.

### Modified Capabilities

None. The project has no existing specs; this is a new set of capabilities.

## Impact

- New workflows under `.github/workflows/` and a composite action under
  `.github/actions/`, all pinned to immutable action references.
- New project configuration at `.opencode/opencode.jsonc` and phase agents
  under `.opencode/agents/`.
- New helper scripts, issue and PR templates, a label setup script, docs under
  `docs/`, and an entry in `.agents/skills/`.
- Updates to `AGENTS.md` and `README.md`; derived repos add `spec-validate` to
  `just ci` and adopt branch protection.
- New repository configuration for Tailscale WIF: the federated identity
  client ID and, when the generated audience is not used, its audience. The
  phase jobs grant `id-token: write`; no long-lived Tailscale secret is
  required. The tailnet policy authorizes the `tag:ci` identity with a base role
  plus model and MCP capabilities, so no model API key is needed when those are
  granted.
- New CI dependencies: the OpenSpec CLI and a pinned OpenCode binary, installed
  by the repository's own composite action. The template's application `just`
  stubs remain unchanged until a derived project fills them in.
- No application code changes and no runtime service; the factory is entirely
  GitHub Actions, repository configuration, and OpenSpec artifacts.
