---
name: github-factory
description: Operate or extend the GitHub-native OpenSpec factory. Use when changing the factory workflows, phase agents, labels, issue and PR templates, factory scripts, or the factory docs.
paths:
  - ".github/**"
  - ".opencode/agents/**"
  - "scripts/factory-*.sh"
  - "docs/factory.md"
---

# GitHub factory conventions

The factory is the default feature workflow for repositories created from this
template. One issue maps to one OpenSpec change, `gh<issue>-<slug>`, and to
three pull requests: proposal, implementation, archive.

## Directives

1. **Phase starts are human command labels; merges drive lifecycle.** Only
   `opsx:propose`, `opsx:implement`, and `opsx:archive` start work. Advance
   lifecycle labels from `pull_request.closed` + `merged`, never by applying a
   command label from the workflow. GitHub suppresses runs started by the
   default `GITHUB_TOKEN`, so a bot-applied trigger label would need a
   long-lived token and loop guards.

2. **The agent never performs a GitHub side effect.** The workflow commits,
   pushes, labels, comments, and opens PRs. Checkout uses
   `persist-credentials: false`; the agent step gets no `GITHUB_TOKEN` and has
   no `gh`. A prompt injection in issue or review text must not be able to
   merge, label, or push.

3. **The control plane is read-only to every agent.** Deny edits under
   `.github/`, `.opencode/`, `.agents/`, `.githooks/`, and `.git/` in the agent
   permissions and reject any such diff in the workflow. An agent that needs a
   control-plane change reports `blocked`; a human makes it. Otherwise a
   compromised run could rewrite its own permissions.

4. **Keep running labels additive.** Add `opsx:proposing`,
   `opsx:implementing`, `opsx:archiving`, `opsx:verifying`, or
   `opsx:revising` beside the lifecycle label; remove the overlay in
   `if: always()`. Never encode lifecycle state in the overlay, or a crashed
   run erases the state that says where the change is.

5. **Only the archive PR closes the issue.** Proposal and implementation PRs
   use `Refs #<issue>`; the archive PR uses `Closes #<issue>`. The issue must
   stay open through code review so `/revise` can still act on an active
   change, and it must close exactly when the change is archived.

6. **Archive is deterministic and model-free.** Run
   `openspec archive <change> --yes`, validate, commit, and open the PR. Do not
   route archiving through an agent and do not join the tailnet for it.

7. **Pin what runs.** Pin GitHub actions by commit SHA, OpenCode by version
   (and verify its release checksum), and install OpenSpec as
   `@fission-ai/openspec@2`. A branch name or floating tag is not a stable
   reference for a workflow that holds secrets.

8. **A failure must be visible and retryable.** Add `opsx:blocked`, post the
   reason and run link, remove the running overlay, and keep the lifecycle
   label. The retry is the same command label after removing `opsx:blocked`.
   The guard re-runs every precondition.

9. **Tailscale uses WIF, not a stored secret.** The phase job grants
   `id-token: write`, the Tailscale step exchanges a short-lived GitHub OIDC
   token through a federated identity, and the repository stores only the
   non-secret client ID and audience. Match the federated identity's subject to
   the exact `sub` prefix the smoke workflow prints, including GitHub's
   immutable owner/repository IDs. The agent step must unset
   `ACTIONS_ID_TOKEN_REQUEST_URL` and `ACTIONS_ID_TOKEN_REQUEST_TOKEN` as well
   as `GITHUB_TOKEN`, because a process that can mint an OIDC token can join
   the tailnet without the MCP permission boundary. Keep `id-token: write` on
   the phase job, not the whole workflow. The tailnet ACL only makes the host
   reachable; a tag-only `tag:ci` identity also needs the tailnet policy's
   Aperture app capability with `role: user` plus model access. Without the
   role, `/api/wif/claims` and the model endpoints return 403
   `access denied: no role granted`. The working `tag:pr-agent` grant shows the
   shape; the Aperture gateway config is a second source, so the policy is the
   reviewable source of the role. The smoke preflight uses a one-token chat
   completion rather than the role-gated catalog.

## Where the contracts live

| Contract | File |
|---|---|
| Lifecycle and labels | `openspec/specs/factory-lifecycle/spec.md`, `docs/factory.md` |
| Agent write, shell, MCP, and result rules | `openspec/specs/factory-agent-runtime/spec.md`, `.opencode/agents/openspec-*.md` |
| Tailnet, model, and MCP access | `openspec/specs/factory-connectivity/spec.md`, `.opencode/opencode.jsonc`, `.github/actions/openspec-agent/action.yml` |
| Label names and colors | `scripts/factory-labels.sh` |
| Status comment | `scripts/factory-status.sh` |
| Context and result formats | `scripts/factory-context.sh`, `scripts/factory-result.sh` |
| Change naming | `scripts/factory-slug.sh` |

The main specs above are created when the `add-agentic-factory` change is
archived. Until then, the same contracts are in
`openspec/changes/add-agentic-factory/specs/`.

If a label is renamed, change the script, the workflows, and `docs/factory.md`
in the same commit. If a phase boundary changes, change the agent file, the
workflow's post-run scope check, and the spec together.

## Checks

```sh
bash -n scripts/*.sh
shellcheck scripts/*.sh
actionlint
openspec validate --all
just ci
```

`actionlint.yaml` ignores ShellCheck `SC2016` because the workflow messages
contain literal Markdown backticks. Do not use that exception for real shell
bugs.

The full lifecycle can only be exercised in a derived repository with a real
application, real `just` checks, the Tailscale WIF variables, the ACL, and the
labels. `base` itself validates the template scaffolding.
