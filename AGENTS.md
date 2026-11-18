# <project>

TODO: one paragraph — what this is, who it is for, and what it explicitly does
not do.

This repository started from the `base` template. It carries a GitHub-native
OpenSpec factory as the default feature workflow: an issue opens a change, and
three reviewed pull requests — proposal, implementation, and archive — land it.
See `docs/factory.md` before changing the workflows, agents, labels, or
templates.

`docs/` holds project documentation. `openspec/` holds change artifacts.

## Commands

Everything runs through `just`.

| Command | Does |
|---|---|
| `just` | List recipes |
| `just fmt` | Format check; fails if anything is unformatted |
| `just lint` | Lint and static analysis |
| `just build` | Build |
| `just test` | Test suite |
| `just pre-commit` | Hook phase: `fmt lint` |
| `just pre-push` | Hook phase: `build test` |
| `just ci` | What CI runs |
| `just install-hooks` | Point git at `.githooks` (once per clone) |
| `just spec-list` | List OpenSpec changes |
| `just spec-validate` | Validate every spec and change |
| `just check-template` | Template-only scaffolding check (`base` uses this as `ci`) |

The four checks are stubs until the project fills them in. Run `just ci` before
reporting any task complete. If a check fails, fix the code, not the check.

A derived project switches to `ci: pre-commit pre-push spec-validate` once its
real checks exist. `base` keeps `ci: check-template` because it has no
application to build or test.

## Hooks

Run `just install-hooks` once per clone. After that `git commit` runs `fmt` and
`lint`, and `git push` runs `build` and `test`. Do not bypass hooks with
`--no-verify`; a broken hook is a bug to fix.

## Capturing learnings

When review feedback, a bug, a failed attempt, or a debugging detour reveals a
convention that should hold beyond the immediate change, record it as a
versioned skill in `.agents/skills/<name>/SKILL.md`. Never leave it only in an
agent's personal memory: that store is machine-local and invisible to other
people, machines, and agents.

Capture it when:

- **A failure cost real time.** Record the non-obvious cause so the next agent
  does not rediscover it from scratch.
- **New work establishes a pattern.** A package boundary, naming convention, or
  cross-cutting wiring decision belongs in a skill so later work stays
  consistent instead of drifting back to ad hoc code.
- **Review feedback generalizes.** If the lesson applies beyond this diff, it is
  a convention.

A one-off fix that does not generalize is just fixed. A skill needs both the
directive and a short rationale; a rule with no *why* is easy to silently
violate or "correct" back to the thing it was written to prevent. Fold a new
learning into an existing skill when the topic fits, and create a new file only
when it does not. Scope a skill with `paths:` frontmatter when it only applies
to part of the tree.

## Feature lifecycle (OpenSpec)

Feature-shaped work — not a one-line fix — moves through OpenSpec. Artifacts
live in `openspec/changes/<name>/`; completed changes are archived.

| Phase | Artifact | Command |
|---|---|---|
| Propose | `proposal.md` — why, what, capabilities | `/opsx-new` or `/opsx-propose` |
| Spec | `specs/<capability>/spec.md` — requirements and scenarios | `/opsx-continue` |
| Design | `design.md` — decisions and trade-offs | `/opsx-continue` |
| Tasks | `tasks.md` — implementation checklist | `/opsx-continue` |
| Implement | code; run `just ci` | `/opsx-apply` |
| Verify | `just spec-validate` plus a review pass | `/opsx-verify` |
| Archive | move the change under `openspec/changes/archive/` | `/opsx-archive` |

Specs use `SHALL` / `MUST` and `#### Scenario:` blocks; `just spec-validate`
must pass before archiving. The proposal and spec need a human accept/reject
before implementation starts — the gate is the approval, not the file.

### GitHub factory

For feature work in a repository created from this template, the GitHub
factory is the default path. It is the same OpenSpec lifecycle with GitHub as
the control plane:

1. File one issue for one cohesive change (`gh<issue>-<slug>`). Split an issue
   that needs several changes.
2. A maintainer applies `opsx:propose`. The factory opens a proposal PR. Review
   the spec and merge it; the issue becomes `opsx:proposal-approved`.
3. A maintainer applies `opsx:implement`. The factory implements the tasks,
   runs `just ci`, and opens an implementation PR. Review the code and merge
   it; the issue becomes `opsx:impl-approved`.
4. A maintainer applies `opsx:archive`. The factory archives the change, syncs
   the specs, and opens the archive PR with `Closes #<issue>`. Merge it; the
   issue closes and becomes `opsx:done`.

The factory is PR-based by design, for feature work only. It is opt-in for
existing repositories; new repositories from this template inherit it. Setup —
labels, the Tailscale WIF client ID/audience, the tailnet policy role/model/MCP
grant, and branch protection — is in `docs/factory.md`. The agent never has a
GitHub token or an OIDC token and cannot edit the factory's own control plane.
Tailscale access uses a GitHub OIDC token exchanged through a federated
identity (`id-token: write`); there is no long-lived Tailscale secret.

## Workflow

Feature work goes through the factory's three pull requests. Outside the
factory, commit directly to `main` only for small, self-contained changes that
do not cross a review gate: one bug fix, one maintenance edit, one refactor.
Keep each commit revertable on its own so `git bisect` and `git revert` stay
useful.

An OpenSpec change still lands as two commits on `main` — the implementation
and the archive — plus the proposal planning commit before them. The factory
produces those as squash-merged PRs:

1. `<change>: implement ...`
2. `<change>: archive and sync specs to main` — only the moves under
   `openspec/` plus the updated `openspec/specs/`.

Do not bypass the factory for feature work merely because it is faster to push
straight to `main`; the gates are the point. If the project deliberately drops
a gate, update this section and `docs/factory.md` in the same change.

## Conventions

TODO as the project acquires them. Keep this list short and push detail into
`.agents/skills/`: comment discipline, dependency policy, error handling,
naming, and anything a reviewer has had to say twice.

The factory's own conventions live in `.agents/skills/github-factory/SKILL.md`.
Treat the factory control plane — `.github/`, `.opencode/agents/`, and the
`factory-*.sh` scripts — as reviewed infrastructure: change it deliberately,
with `actionlint`, the script checks, and `openspec validate --all`, not as a
side effect of feature work.
