# GitHub-native OpenSpec factory

The factory turns one GitHub issue into one OpenSpec change and lands it
through three reviewed pull requests: proposal, implementation, and archive.
GitHub is the control plane. Humans keep every judgment: they start each
phase, review each pull request, and merge it. The model does the mechanical
work and never touches GitHub.

```
issue ──opsx:propose──▶ proposal PR ──merge──▶ opsx:proposal-approved
      ──opsx:implement──▶ implementation PR ──merge──▶ opsx:impl-approved
      ──opsx:archive──▶ archive PR ──merge──▶ issue closed, opsx:done
```

The same issue and change name, `gh<issue>-<slug>`, appear on every branch,
pull request, and archive path. One issue maps to exactly one change; split
anything larger before applying a command.

## Setup in a repository created from this template

1. **Create the repository** from the template, then run `just install-hooks`
   once per clone.
2. **Create the labels**:
   ```sh
   ./scripts/factory-labels.sh --repo OWNER/NAME
   ```
   The script is idempotent; `gh label create --force` updates colors and
   descriptions.
3. **Create a Tailscale federated identity** for GitHub Actions WIF. In the
   Tailscale admin console, open **Trust credentials → Credential → OpenID
   Connect**, choose the GitHub Actions issuer (`https://token.actions.githubusercontent.com`),
   and set the subject to the exact `sub` prefix printed by the smoke workflow,
   with a wildcard for the ref. GitHub may issue either
   `repo:OWNER/NAME:*` or an immutable subject such as
   `repo:OWNER@OWNER_ID/NAME@REPO_ID:*`; copy the prefix the smoke run shows
   rather than guessing. Grant the `auth_keys` scope and allow `tag:ci`. Copy
   the **Client ID** and **Audience**. Then add the repository variable
   `TS_CLIENT_ID`; add `TS_AUDIENCE` only if your identity uses a custom
   audience (otherwise the action derives `api.tailscale.com/<client-id>`).
   These values are not secrets. Do not create `TS_OAUTH_SECRET`; delete it if
   it exists. No model API key is needed when the tailnet policy grants the CI
   identity a base role and model access.
4. **Grant the CI tag access in the tailnet policy.** The CI tag needs both
   network reachability and the Aperture app capability in the same grant. In
   this environment the policy is `tailnet/policy.hujson` in the infra repo;
   add:
   ```json
   {
     "src": ["tag:ci"],
     "dst": ["tag:ai"],
     "ip":  ["tcp:443"],
     "app": {
       "tailscale.com/cap/aperture": [
         { "role": "user", "enable_portal": false },
         {
           "models": "*/deepseek-v4.1-flash",
           "quotas": [{ "bucket": "ci-agents" }]
         }
       ]
     }
   }
   ```
   Replace `tag:ai` with the Aperture host's tag, or use `ai` as the
   destination if your policy uses hostnames. Do not grant SSH or any other
   tailnet host. Tailscale trims the netmap, so without the network grant the
   runner will not even see `ai` in `tailscale status`. The role and model
   capability belong in this policy grant, not only in the Aperture gateway
   config; Aperture combines both sources, and the policy keeps the role
   reviewable in git. Update the policy's `tag:ci` test to accept `tag:ai:443`
   instead of denying it. Without the base role, `/api/wif/claims` and the model
   endpoints return
   `403 {"error":{"message":"access denied: no role granted"}}`. The role and
   model access are separate. If propose or verify use MCP web tools, also
   grant the read-only web connectors (for example `aperture_web_search` and
   `aperture_web_fetch`; use the names shown by Aperture).
5. **Protect `main`**: require a pull request and the `CI` check before
   merging. Do not let workflows push to `main`; the factory pushes only phase
   branches and opens PRs.
6. **Allow GitHub Actions to create pull requests** in the repository
   settings. The workflows use `GITHUB_TOKEN` with `pull-requests: write`;
   they never approve or merge.
7. **Run the smoke workflow** (`Actions → OpenSpec smoke → Run workflow`). It
   prints the non-secret `iss`, `aud`, `sub`, `repository`, `ref`,
   `event_name`, and `workflow` claims, exchanges the GitHub OIDC token for a
   short-lived Tailscale credential, joins as `tag:ci`, makes a one-token chat
   completion with `deepseek-v4.1-flash`, and stops before installing or
   running the agent. If the exchange is rejected, match the federated
   identity's subject to the printed `sub` prefix. If the preflight fails after
   the node joins, check the tailnet ACL, the policy's role/model capability,
   and the `/etc/hosts` mapping described below.
8. **Add `spec-validate` to `just ci`** once the project has real checks:
   `ci: pre-commit pre-push spec-validate`. The template's own `ci` stays
   `check-template`, because `base` has no application to build or test.

The optional `openspec-janitor.yml` workflow runs daily, needs no secrets, and
removes stale running labels.

### Tailscale WIF configuration

The phase jobs grant `id-token: write`; the agent step unsets
`ACTIONS_ID_TOKEN_REQUEST_URL` and `ACTIONS_ID_TOKEN_REQUEST_TOKEN`, so the
model never receives the OIDC request variables. The Tailscale step uses the
federated identity client ID and audience, exchanges the OIDC token, and
creates an ephemeral node. The preflight maps the Aperture FQDN to the peer's
tailnet IP in `/etc/hosts`, because GitHub runners do not always wire MagicDNS
into systemd-resolved; that mapping also lets the agent resolve the provider
and MCP endpoints.

The same identity can be created with the Tailscale Terraform provider:

```hcl
resource "tailscale_federated_identity" "factory" {
  description = "GitHub Actions OpenSpec factory"
  scopes      = ["auth_keys"]
  tags        = ["tag:ci"]
  issuer      = "https://token.actions.githubusercontent.com"
  # Replace with the exact `sub` prefix printed by the smoke workflow.
  subject     = "repo:OWNER@OWNER_ID/NAME@REPO_ID:*"
}
```

Use the `sub` prefix from the smoke workflow's claims output, followed by
`:*`. GitHub's immutable subject form includes the owner and repository IDs,
so the prefix can look like
`repo:kevindurb@2565006/base@1448466354`; a plain `repo:OWNER/NAME:*` works only
when GitHub emits that shorter form. The prefix printed by the smoke run is
authoritative. It covers default-branch runs (`issues`, `issue_comment`,
`workflow_dispatch`) and `pull_request` runs. For a tighter policy, use two
identities with subjects `<prefix>:ref:refs/heads/main` and
`<prefix>:pull_request`, and give each one its own client ID. A custom claim
matching `repository` can be added as defense in depth.

If your global OpenCode configuration pins a different provider ID or denies
other providers, allow `aperture` in that policy before running the propose
agent locally; CI has no global configuration to override the project config.

## Commands and labels

Only command labels start a phase. A maintainer (write, maintain, or admin
permission) applies one to an open issue. The phase removes the command label
before it starts.

| Label | Kind | Meaning |
|---|---|---|
| `opsx:propose` | command | Start planning from the issue |
| `opsx:implement` | command | Build the approved change |
| `opsx:archive` | command | Archive the implemented change |
| `opsx:proposing` | running overlay | A propose run is in progress |
| `opsx:implementing` | running overlay | An implementation run is in progress |
| `opsx:archiving` | running overlay | An archive run is in progress |
| `opsx:verifying` | running overlay (PR) | An advisory verification is running |
| `opsx:revising` | running overlay (PR) | A revise run is running |
| `opsx:proposal-review` | lifecycle | Proposal PR is open for review |
| `opsx:proposal-approved` | lifecycle | Proposal merged; `opsx:implement` is next |
| `opsx:impl-review` | lifecycle | Implementation PR is open for review |
| `opsx:impl-approved` | lifecycle | Implementation merged; `opsx:archive` is next |
| `opsx:archive-review` | lifecycle | Archive PR is open for review |
| `opsx:done` | lifecycle | Archived and complete |
| `opsx:blocked` | failure | The last phase stopped; read the reason before retrying |
| `opsx:triage` | triage | The issue form's initial label, removed when a phase starts |

Running labels are additive. They never replace the lifecycle label, so a
crashed run leaves the lifecycle state visible and the overlay as an obvious
anomaly. Every phase removes its overlay in an `if: always()` step; the
janitor is the second line of defense.

Lifecycle labels advance on merge events, never on a bot-applied command
label. That is deliberate: GitHub suppresses workflows started by the default
`GITHUB_TOKEN`, so a bot-applied command label would need a long-lived token.
Merges are human events and always trigger the lifecycle workflow.

## The three pull requests

| Phase | Head branch | Body reference | Closes the issue? |
|---|---|---|---|
| Proposal | `opsx/propose-<change>` | `Refs #<issue>` | No |
| Implementation | `opsx/impl-<change>` | `Refs #<issue>` | No |
| Archive | `opsx/archive-<change>` | `Closes #<issue>` | Yes |

**Proposal.** Creates `openspec/changes/<change>/` with every artifact the
schema requires and nothing else. The workflow validates the change, checks
that the diff contains only that directory, commits, pushes, and opens the
PR. The issue moves to `opsx:proposal-review`. If the agent blocks after
creating useful artifacts, the workflow may open a **draft** proposal PR so
the partial work is reviewable, adds `opsx:blocked`, and never creates a second
change.

**Implementation.** Requires `opsx:proposal-approved` and exactly one active
change for the issue. The agent edits project code and the active change,
checks tasks only when their behavior is complete, runs `just ci`, and leaves
the change active. The workflow validates the change, rejects any
control-plane edits, commits, pushes, and opens the implementation PR. The
issue moves to `opsx:impl-review`. If `just ci` fails, the branch is pushed for
recovery but no PR is opened or updated, and the issue is `opsx:blocked`.

**Archive.** Requires `opsx:impl-approved`, a merged implementation PR, an
active change, and no open archive PR. It runs
`openspec archive <change> --yes`, which moves the change to
`openspec/changes/archive/<date>-<change>/` and syncs its specs, then runs
`openspec validate --all --strict` and `openspec validate --archived --strict`.
It commits the move and opens the archive PR whose body says `Closes #<issue>`.
It never calls the model and never joins the tailnet. A validation failure
opens no PR and marks the issue blocked. The issue moves to
`opsx:archive-review`; merging closes the issue and the lifecycle workflow sets
`opsx:done`.

The archive guard looks for both `gh<issue>-*` and `*-gh<issue>-*` under
`openspec/changes/archive/`, so an already archived change is reported by its
real dated path and no duplicate is created.

## Failure and retry

A failed or blocked phase:

- keeps the lifecycle label;
- adds `opsx:blocked`;
- posts the reason and the run link;
- removes its running overlay.

To retry, fix the cause, remove `opsx:blocked`, and re-apply the same command
label. The guard runs again: it re-checks permissions, lifecycle state,
duplicates, and open PRs before starting. The archive workflow's retry path
is the same label; it does not need a new issue.

The janitor removes a running overlay that no run has refreshed for 90
minutes, comments the retry command, and adds `opsx:blocked` on issues. It
never deletes a pull request or a commit.

## Verify and revise

`openspec-verify.yml` runs on implementation PRs and can also be dispatched
with a PR number. It reads the change, the specs, and the PR diff, and posts a
**comment-only** review. It never approves or requests changes, and it has no
shell or GitHub write credential in the agent step. CI remains the test gate.
Verification is advisory.

`openspec-revise.yml` handles a maintainer's `/revise` comment and a
changes-requested review. It picks the revise agent, feeds the review text in
as untrusted data, and pushes one fix commit to the same PR branch. On a
proposal PR its writes are limited to the change artifacts; on an
implementation PR it may edit code and the change but never the control
plane. Verify and revise share a `factory-pr-<number>` concurrency group with
`cancel-in-progress: false`, so a second request waits or exits rather than
duplicating comments or commits.

## Agent boundaries

The prompt is static. Everything event-specific reaches the agent through
`.agent/context.md`, and issue, PR, review, and comment text is fenced as
untrusted data. The agent writes `.agent/result.json` and `.agent/report.md`;
the workflow validates the result before it publishes anything.

| Phase | Agent | Writes | Shell | MCP |
|---|---|---|---|---|
| Propose | `openspec-propose` | Change directory and `.agent/` | `openspec`, read-only git | read-only web search and fetch |
| Apply | `openspec-apply` | Project code and the active change | `just`, `openspec`, read-only git | none |
| Verify | `openspec-verify` | `.agent/` only | none | read-only web search and fetch |
| Revise | `openspec-revise` | Project code and the change | `just`, `openspec`, read-only git | none |

Every agent denies by default, denies subagents and questions, and denies the
factory control plane: `.github/`, `.opencode/`, `.agents/`, `.githooks/`, and
`.git/`. The dangerous Aperture tools (`TailnetSSH_*`, `chrome_*`, session
inspection, and MCP resource reads) are denied explicitly. Raw network
clients are not in the shell allowlist, so an agent cannot bypass the MCP
permission boundary by calling the endpoint directly. Agents have no GitHub
token, no `gh`, and no persisted checkout credential. The agent step also
unsets the GitHub OIDC request variables, so it cannot mint or exchange a
token even though the surrounding phase job has `id-token: write`.

The result contract is:

```json
{
  "status": "ok | blocked | failed",
  "phase": "propose | apply | verify | revise",
  "change": "gh<issue>-<slug>",
  "summary": "one-line outcome",
  "blocked_reason": "required when status is blocked",
  "verdict": "pass | concerns (verify only)"
}
```

A missing or invalid result is a failed phase. It never opens a PR and never
updates one.

## Configuration map

| Path | Purpose |
|---|---|
| `.opencode/opencode.jsonc` | Aperture provider, `aperture/deepseek-v4.1-flash` limits, remote MCP without OAuth or Code Mode |
| `.opencode/agents/openspec-*.md` | Phase prompts, step limits, deny-by-default permissions |
| `.github/actions/openspec-agent/action.yml` | Ephemeral Tailscale node, bounded model preflight, pinned OpenCode and OpenSpec, agent run |
| `.github/workflows/openspec-propose.yml` | Issue command, guards, proposal branch and PR |
| `.github/workflows/openspec-apply.yml` | Implementation branch and PR, `just ci`, control-plane deny |
| `.github/workflows/openspec-archive.yml` | Deterministic archive, validation, `Closes` PR |
| `.github/workflows/openspec-lifecycle.yml` | Merge-driven lifecycle labels and status comment |
| `.github/workflows/openspec-verify.yml` | Advisory, comment-only verification |
| `.github/workflows/openspec-revise.yml` | `/revise` and changes-requested revisions |
| `.github/workflows/openspec-janitor.yml` | Daily stale-overlay sweep |
| `.github/workflows/openspec-smoke.yml` | Dispatchable WIF and model-endpoint preflight |
| `.github/ISSUE_TEMPLATE/change.yml` | One-change issue form |
| `.github/PULL_REQUEST_TEMPLATE.md` | Three-PR reference and `Refs` reminder |
| `scripts/factory-context.sh` | Build `.agent/context.md` with untrusted fences |
| `scripts/factory-result.sh` | Validate the result contract |
| `scripts/factory-status.sh` | Create or update the single status comment |
| `scripts/factory-labels.sh` | Idempotent label setup |
| `scripts/factory-slug.sh` | Derive `gh<issue>-<slug>` |

The OpenCode binary is pinned to `2.2.5` and verified against the release's
`SHA256SUMS`; OpenSpec is installed as `@fission-ai/openspec@2`. Every GitHub
action is pinned to a commit SHA. Update those pins deliberately, not with a
floating tag.

## Testing

`base` is a template: its `just ci` is `check-template`, so the full lifecycle
cannot run here. Test the factory in a repository created from the template,
with a real application and real `fmt`, `lint`, `build`, and `test` recipes.

Useful local checks before pushing a factory change:

```sh
bash -n scripts/*.sh
shellcheck scripts/*.sh
actionlint
openspec validate --all
just ci
```

After the WIF setup, run the **OpenSpec smoke** workflow. It exchanges the
OIDC token, joins as `tag:ci`, makes a one-token completion with the configured
model, and stops before installing or running the agent. A failure there is a
federated identity, audience, variable, ACL, or tailnet policy role/model-grant
problem, not a model problem.

The propose agent can be smoke-tested locally with a hand-written
`.agent/context.md` and an ephemeral node, or by running the same command from
a machine already on the tailnet. If your global OpenCode configuration pins a
different provider ID or denies other providers, allow `aperture` in that
policy (or run the test with a clean config home); the project config defines
`aperture/deepseek-v4.1-flash`, and CI has no global configuration to
override it.
