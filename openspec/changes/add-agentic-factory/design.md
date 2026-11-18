# Design

## Context

See `proposal.md` for motivation and `specs/` for the behavior contract.

`base` is a template, not an application: `just` exposes a command contract whose
`fmt`, `lint`, `build`, and `test` recipes are stubs, `.githooks` calls those
recipes, `openspec/` holds change artifacts, and `.opencode/` holds OpenSpec
commands and skills. CI currently runs `just ci`, which in the template is
`check-template`.

The factory is entirely GitHub Actions plus repository configuration. There is
no server, database, or long-running process. The relevant constraints are:

- GitHub-hosted runners are ephemeral, have outbound network only, and start
  without the OpenSpec or OpenCode CLIs.
- The Aperture host is reachable only through Tailscale, authorizes requests
  by tailnet identity, and depends on the tailnet policy for the base role plus
  model/MCP capabilities. It serves an OpenAI-compatible model plus an MCP
  endpoint.
- No available third-party OpenCode action runs OpenCode V2 for general phase
  work; the factory uses its own composite action with a pinned,
  checksum-verified OpenCode binary.
- `AGENTS.md` currently says to commit directly to `main` and land a change as
  two commits; the factory is PR-based, so that section must be updated.
- The template's `just` stubs mean the factory cannot be exercised end to end
  in `base` itself; it must be tested in a repository created from the template.

## Goals / Non-Goals

**Goals:**

- Make the issue the single control plane and the OpenSpec change the durable
  state, with three reviewed PRs per change.
- Keep every GitHub side effect in deterministic workflow code, never in the
  model.
- Make phase transitions visible through labels and a status comment, and make
  failure recoverable with a command label.
- Run the agent against the private Aperture endpoint with the smallest
  practical tailnet, tool, shell, and filesystem surface.
- Keep the template usable: a derived repository inherits the factory, while
  `base` itself remains a scaffolding check.

**Non-Goals:**

- Auto-merging any PR, autonomous issue triage, a quick-fix lane that bypasses
  OpenSpec, multi-repo orchestration, or a custom dashboard.
- A GitHub App or long-lived bot identity; `GITHUB_TOKEN` is sufficient because
  PR events drive automatic transitions.
- A model fallback or a second provider. If Aperture is down, the phase blocks.

## Decisions

### 1. Three PRs per change, not one

One proposal PR, one implementation PR, one archive PR, all linked to the same
issue. The proposal diff is pure planning, so the spec review is not buried in
code. The implementation PR stays against an active change, so review feedback
can still be applied. The archive PR is the only PR that says `Closes #N`, so
the issue closes exactly when the change is fully archived.

Alternatives considered: one PR with a planning commit and an approval label
(collapses the spec and code gates, noisy diff); two PRs with the archive commit
inside the implementation PR (the change is archived before review, so spec
feedback is awkward).

### 2. The issue is the control plane; the change name embeds its number

Change names are `gh<issue>-<slug>`, for example `gh42-add-oauth-login`. The
number is parseable from the branch, the archive path, and the PR head, so no
separate mapping table is needed. One issue maps to exactly one change; an
issue that needs several changes is split.

Alternatives considered: a free-form slug plus metadata in the issue body
(linkage drifts when edited); a PR as the control plane (PRs are transient and
lose the lifecycle when closed).

### 3. Three label roles: command, running overlay, lifecycle

Only command labels (`opsx:propose`, `opsx:implement`, `opsx:archive`) start a
phase. Running labels (`opsx:proposing`, `opsx:implementing`,
`opsx:archiving`, `opsx:verifying`, `opsx:revising`) are additive overlays, so
a crashed run cannot erase the lifecycle state; a stale overlay is a visible
anomaly. Lifecycle labels record where the change is, and `opsx:blocked` is a
failure overlay.

Alternatives considered: a single `opsx:running` label (loses which phase is
running); using lifecycle labels as triggers (a bot-applied label can retrigger,
and the label means two things); no labels (the issue list stops being a
dashboard).

### 4. PR merge events drive automatic transitions; labels start phases

A proposal merge advances the issue to `opsx:proposal-approved`; an
implementation merge advances it to `opsx:impl-approved`; an archive merge sets
`opsx:done`. Those transitions are event-driven and do not need a bot to apply a
trigger label. Starting a phase is always a human command label.

This is deliberate because GitHub suppresses workflow runs caused by the
default `GITHUB_TOKEN` (except `workflow_dispatch` and `repository_dispatch`).
If the factory used a bot-applied label to start the next phase, it would need
a PAT or App token and loop guards. PR events avoid both.

Alternatives considered: a bot applies the next command label with a PAT (adds
a long-lived credential and loop risk); one dispatcher workflow invokes every
phase (more coupling and harder logs).

### 5. The agent never touches GitHub

The agent edits the working tree and writes two files; the workflow commits,
pushes, opens PRs, labels, and comments. Checkout uses
`persist-credentials: false`, and `GH_TOKEN` is present only in the push and
comment steps. The agent has no `gh`, no push credential, and no approval
authority.

Alternatives considered: give the agent `gh` (a prompt injection could label,
merge, or comment); let the agent commit and push (requires credentials in the
agent step and makes retries non-idempotent).

### 6. Static prompt plus a file-based context and result contract

The composite action's prompt is a fixed string that points at
`.agent/context.md`. Event data is written to that file by the workflow, with
issue and review text fenced as untrusted. The agent writes
`.agent/result.json` and `.agent/report.md`; the workflow validates the result
and uses the report as the PR or comment body. Every run is stateless, so no
session continuation is needed.

Alternatives considered: interpolate issue text into the CLI prompt (shell
injection and quoting problems); parse the agent's prose (fragile); rely on
OpenCode session history (CI runs are fresh and sessions are not shared).

### 7. Deny-by-default phase agents; the control plane is read-only

Each agent is a named primary agent with a broad deny rule followed by narrow
allows. Propose may write only the change directory and `.agent/`; apply and
revise may write code and the active change but never `.github/`, `.opencode/`,
`.agents/`, `.githooks/`, or `.git/`; verify may write only `.agent/`. Shell
access is limited to `openspec`, `just`, and read-only git as appropriate, and
raw network clients are denied so the agent cannot bypass MCP permissions by
calling the endpoint directly. MCP web tools are allowed for propose and verify
and denied for apply and revise.

Alternatives considered: one general agent with a phase prompt (too broad); rely
on prompt instructions alone (not enforced); allow control-plane edits and rely
on review (a compromised run could rewrite its own permissions).

### 8. Verify is advisory and shell-less; archive is model-free

The verify agent reads the PR diff, the change artifacts, and `REVIEW.md`, and
posts a comment-only review. It has no shell, so it never executes PR code on a
tailnet-connected runner; CI is the test gate. Archiving is a deterministic CLI
job (`openspec archive --yes`, validation, commit, PR) that does not use the
model or join the tailnet.

Alternatives considered: verify runs `just ci` (executes untrusted code with
secrets); archive via an agent (unnecessary cost and risk); make verification a
required check (false positives would block merges).

### 9. Archive is a separate PR, not a commit on `main`

The archive PR contains the move to `openspec/changes/archive/` and the synced
`openspec/specs/`. It gives the spec sync a review surface, keeps `main`
protected against bot pushes, and makes `Closes #N` close the issue at the true
end. The change is still active while the implementation PR is reviewed, so
`/revise` works normally.

Alternatives considered: a bot commit to `main` (bypasses branch protection and
has no review); archive inside the implementation PR (the change is archived
before review, and a revise after that is awkward).

### 10. Ephemeral Tailscale node, HTTPS, pinned action and OpenCode versions

Each workflow that needs the model joins the tailnet as an ephemeral `tag:ci`
node using a short-lived GitHub OIDC token exchanged through a Tailscale
federated identity (WIF), then preflights the model with a one-token chat
completion and bounded retries. The phase job grants `id-token: write`; the
agent step unsets the OIDC request variables and never receives a token. The
provider is the OpenAI-compatible endpoint with declared model limits; the MCP
server is remote Streamable HTTP with OAuth disabled and Code Mode disabled.
The action is pinned by commit SHA and OpenCode by version.

Alternatives considered: a long-lived Tailscale OAuth client secret (another
credential to rotate; WIF is short-lived and scoped to the workflow); a public
endpoint (exposes the model and MCP); a self-hosted runner (operational
burden); HTTP (plaintext SSE); `latest` (reproducibility and supply-chain
risk).

### 11. Squash merges, one commit per PR

Each PR carries one logical change, so squashing gives a clean history. The
change still lands as two commits on `main` — `implement` and `archive` — which
is what `AGENTS.md` asks for; the proposal is a planning commit before those.

Alternatives considered: merge commits (preserves agent-internal commits but
adds noise); rebase (fine, but unnecessary for one-commit PRs).

### 12. Build into `base`, test in a derived repository

The template carries the factory so every new repository inherits it, and the
factory is exercised in a scratch repository created from the template. `base`
keeps `ci: check-template`; a derived project switches to
`ci: pre-commit pre-push spec-validate`.

Alternatives considered: a separate factory repository (version skew and extra
setup); copying the workflows into each project (drift).

## Risks / Trade-offs

- **Prompt injection from issue or review text** → human merge, no auto-merge,
  no GitHub token in the agent, control-plane deny, untrusted delimiters, and
  human review of the final diff.
- **The MCP is reachable by the CI grant and includes `TailnetSSH_run_command`
  and Chrome** → the tailnet policy limits `tag:ci` to the host on 443, grants
  only the model and MCP capabilities, agents deny those tools, apply and
  revise get no MCP, and shell egress is denied.
- **Five human actions may feel heavy** → the work is automated; the gates are
  the point. If it is too much later, the archive label and merge are the first
  to collapse, not the proposal merge.
- **A change can sit active between implementation merge and `opsx:archive`** →
  archive promptly; the label is the deliberate trigger and the archive job is
  cheap. If drift becomes a problem, revisit the gate consciously rather than
  adding a hidden bot path.
- **Stale running labels after a crashed runner** → `if: always()` cleanup plus
  an optional janitor workflow.
- **The model proxy may reject optional OpenAI fields** → the Phase 0 smoke
  test surfaces it; set `compatibility.streamUsage` or `maxTokensField` only if
  needed.
- **`GITHUB_TOKEN` cannot trigger the next phase** → transitions use PR merge
  events; phase starts are human labels.
- **Verification false positives** → advisory comment only; humans decide.
- **WIF subject, audience, or `id-token: write` misconfiguration** → the
  Tailscale action fails before the agent starts; the dispatch smoke workflow
  surfaces the exchange and model-preflight failure without a model call.
- **Netmap trimming hides the Aperture host when the ACL lacks a `tag:ci`
  grant** → the runner cannot even see `ai` in `tailscale status`; add the
  `tag:ci` → `tag:ai` `tcp:443` grant and treat the smoke preflight as the
  setup check.
- **Tailscale or Aperture outage** → preflight fails fast, the phase becomes
  `opsx:blocked`, and re-applying the command retries.
- **The template's `just` stubs block end-to-end testing in `base`** → test in a
  derived scratch repository; keep `base` on `check-template`.
- **Existing repositories created before this change do not inherit it** → the
  README documents opt-in adoption; only new repositories get it automatically.

## Migration Plan

1. Land the planning artifacts and then the implementation in phases.
2. Test Phase 0 locally; test the full lifecycle in a scratch repository created
   from the template.
3. Update `AGENTS.md`, `README.md`, `docs/factory.md`, and the
   `.agents/skills/github-factory` skill in the same change.
4. Rollback is deletion of the workflows, agents, and config plus relaxing
   branch protection; no application data or runtime service is involved.
   Labels can remain without effect.

## Open Questions

- Whether to add a quick-fix lane for one-line fixes after v1.
- Whether to add a GitHub App for bot-driven label transitions or metrics.
- Whether to mirror the verify/revise running labels onto the issue for a single
  dashboard.
- The exact Tailscale tag of the Aperture host; the ACL must match the tag in
  use.
- Whether the proxy needs explicit compatibility settings; the smoke test will
  decide.
