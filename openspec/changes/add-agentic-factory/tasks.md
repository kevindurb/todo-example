# Tasks

## 1. Foundation and smoke test

- [x] 1.1 Add `.opencode/opencode.jsonc` with the Aperture provider, the `deepseek-v4.1-flash` model limits, and the remote MCP server; verify `opencode models` lists `aperture/deepseek-v4.1-flash` and a one-shot `opencode run` reaches the endpoint over the tailnet.
- [x] 1.2 Add the four phase agents under `.opencode/agents/` with deny-by-default permissions, step limits, and the static phase instructions; verify a scratch `opencode run --agent openspec-propose` starts with that agent and a denied write outside the change directory is rejected.
- [x] 1.3 Add `.agent/` to `.gitignore`, then add `factory-context.sh`, `factory-result.sh`, `factory-status.sh`, and `factory-labels.sh`; verify `bash -n` passes for each script and a dry run of the slug derivation produces `gh42-add-oauth-login`.
- [x] 1.4 Add the label setup script with colors and descriptions; verify `gh label list` on a scratch repository shows every command, running, lifecycle, failure, and triage label with the intended color.
- [x] 1.5 Add the change issue form and the pull request template; verify a YAML parser accepts the issue form and the template renders the three-PR references without a closing keyword for proposal and implementation.
- [x] 1.6 Install `@fission-ai/openspec@2` in CI and the composite action, and add `spec-validate` to the documented derived-repo `ci` recipe; verify `openspec --version` and `openspec validate --all` run in a scratch workflow.
- [x] 1.7 Run the propose-agent smoke test in a scratch clone with a hand-written `.agent/context.md`; verify it creates a valid change under `openspec/changes/`, writes `.agent/result.json` and `.agent/report.md`, and cannot run `curl` or write outside its scope.
- [x] 1.8 Add the setup, configuration, label contract, and agent-boundary sections to `docs/factory.md` and add `.agents/skills/github-factory/SKILL.md`; verify the documented label names and commands match the scripts and the spec.

## 2. Propose workflow

- [x] 2.1 Add `.github/actions/openspec-agent/action.yml` with Tailscale WIF, bounded model preflight, OpenSpec install, and the pinned OpenCode binary; add a dispatch smoke workflow; verify the smoke run exchanges the OIDC token and makes a one-token completion with `deepseek-v4.1-flash`.
- [x] 2.2 Add `openspec-propose.yml` with the command trigger, maintainer guard, duplicate guard, context build, proposal branch and PR, lifecycle label, running overlay, and status comment; verify a scratch issue produces a proposal PR containing only `openspec/changes/gh<N>-<slug>/` and passing `openspec validate`.
- [x] 2.3 Add the propose failure paths for an unauthorized actor, a duplicate run, a missing result, and an agent-blocked result; verify each posts a reason, sets `opsx:blocked`, and leaves no `opsx:proposing` label.
- [x] 2.4 Add a draft-PR path for partial propose work; verify a blocked run with useful artifacts leaves a draft PR and does not create a second change.
- [x] 2.5 Document the propose command, its guards, and its recovery in `docs/factory.md`; verify the documented labels and branch prefix match the workflow.

## 3. Implement workflow and lifecycle transitions

- [x] 3.1 Add `openspec-lifecycle.yml` for merged proposal, implementation, and archive PRs; verify a scratch merge moves the issue to `opsx:proposal-approved`, then `opsx:impl-approved`, and that the status comment records the next command.
- [x] 3.2 Add `openspec-apply.yml` with the `opsx:implement` trigger, lifecycle guard, single-active-change check, context build, implementation branch and PR, running overlay, and failure handling; verify a scratch run opens an implementation PR with code and checked tasks and leaves the change active.
- [x] 3.3 Enforce the apply write scope and control-plane deny; verify the agent can edit project code but cannot edit `.github/workflows/`, `.opencode/`, `.agents/`, or `.githooks/`, and reports `blocked` when a change requires them.
- [x] 3.4 Verify the apply agent runs `just ci` and does not archive; verify a failing check produces `opsx:blocked` and no implementation PR update beyond the pushed branch.
- [x] 3.5 Document the implement command, lifecycle transitions, and control-plane boundary in `docs/factory.md`; verify the documented command sequence matches the labels the workflow sets.

## 4. Archive workflow

- [x] 4.1 Add `openspec-archive.yml` with the `opsx:archive` trigger, maintainer guard, merged-implementation check, active-change check, `openspec archive --yes`, validation, archive branch, and PR with `Closes #<issue>`; verify a scratch run moves the change under `openspec/changes/archive/`, syncs `openspec/specs/`, and passes validation.
- [x] 4.2 Add the archive idempotency paths for an already-archived change, an open archive PR, and a missing merged implementation; verify each comments the existing link or precondition and creates no duplicate.
- [x] 4.3 Verify merging the archive PR closes the issue, sets `opsx:done`, and leaves exactly the implementation and archive commits for the change on `main`.
- [x] 4.4 Verify the archive workflow makes no model call and does not join the tailnet; verify a validation failure sets `opsx:blocked` and opens no PR.
- [x] 4.5 Document the archive command, its preconditions, and the `Closes` semantics in `docs/factory.md`; verify the documented recovery path retries with the same label.

## 5. Verify and revise workflows

- [x] 5.1 Add `openspec-verify.yml` with the implementation-PR trigger, same-repo guard, read-only context, `opsx:verifying` overlay, and comment-only review; verify it posts a report, does not approve, and has no shell access.
- [x] 5.2 Add `openspec-revise.yml` for `/revise` and changes-requested reviews with the correct phase agent and write scope; verify it pushes a fix commit to a proposal PR and to an implementation PR without touching the control plane.
- [x] 5.3 Verify verify/revise serialization on the same PR; verify a second request waits or exits and does not create duplicate comments or commits.
- [x] 5.4 Document `/revise`, the advisory nature of verification, and the PR-side running labels in `docs/factory.md`; verify the documented commands match the workflow triggers.

## 6. Integration, hardening, and project docs

- [ ] 6.1 Run the full walkthrough in a fresh repository created from the template: issue → `opsx:propose` → merge → `opsx:implement` → merge → `opsx:archive` → merge; verify the issue closes, all five gates occur, and every label transition is correct.
- [ ] 6.2 Run the failure matrix: Aperture down, Tailscale down, CI failing, duplicate command, non-maintainer command, fork PR, stale running label, and an agent attempting a control-plane edit; verify each ends in `opsx:blocked` or a skipped run with no stale overlay and no unintended PR.
- [x] 6.3 Add the optional janitor workflow for stale running labels; verify it cleans a synthetic stale overlay and leaves a comment that names the retry command.
- [x] 6.4 Update `AGENTS.md` and `README.md` so the factory is the documented default feature workflow, the PR-based deviation from direct-to-main is explicit, and the derived-repo setup lists the Tailscale WIF client ID/audience, ACL, labels, and branch protection; verify `just ci` passes in `base` and `openspec validate --all` passes.
- [ ] 6.5 Verify the security boundaries with a scratch run: the agent has no GitHub token, no `gh`, no persisted checkout credential, no raw network client, and no dangerous MCP tool; verify each attempted escape is denied and reported.
- [x] 6.6 Verify `just ci` in the scratch derived repository runs `fmt`, `lint`, `build`, `test`, and `spec-validate`, and that the template's `base` `ci` still runs only `check-template`.
- [x] 6.7 Switch Tailscale authentication to GitHub WIF: grant `id-token: write` on phase jobs, exchange the OIDC token through a federated identity, remove `TS_OAUTH_SECRET`, add the dispatch smoke workflow, and verify the smoke run completes a one-token preflight with `deepseek-v4.1-flash`.

## Workflow follow-up

- Archive this change after the project's review requirements are satisfied.
- Verify the archived result and the synced specs.
