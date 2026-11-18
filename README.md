# base

The template repo for new projects. It carries the workflow scaffolding I want
in every repo and nothing about a language, build system, or deployment.

Marked as a GitHub template: **Use this template → Create a new repository**.

It also carries a GitHub-native OpenSpec factory: one issue becomes one change,
landed through a proposal PR, an implementation PR, and an archive PR. The
factory is the default feature workflow for repositories created from this
template. See [`docs/factory.md`](docs/factory.md).

## What's here

| Path | Why |
|---|---|
| `justfile` | The command contract. Every repo exposes the same entry points. |
| `.githooks/` | `pre-commit` and `pre-push`, both thin wrappers over `just`. |
| `AGENTS.md` | Canonical guide for coding agents: commands, workflow, conventions. |
| `REVIEW.md` | Generic pre-land review checklist. |
| `.agents/skills/` | Durable, versioned project conventions. One topic per `SKILL.md`. |
| `openspec/` | Spec-driven change artifacts (`proposal → specs → design → tasks`). |
| `.opencode/` | OpenSpec commands, skills, the Aperture provider, and the four phase agents. |
| `.github/workflows/ci.yml` | Runs `just ci` and `openspec validate --all`. |
| `.github/workflows/openspec-*.yml` | The factory: propose, apply, archive, lifecycle, verify, revise, janitor, smoke. |
| `.github/actions/openspec-agent/` | Tailnet, preflight, pinned OpenCode, and one phase agent. |
| `.github/ISSUE_TEMPLATE/`, `.github/PULL_REQUEST_TEMPLATE.md` | One-change issue form and the three-PR reference. |
| `scripts/factory-*.sh` | Context, result, status, labels, and change-name helpers. |
| `docs/factory.md` | Setup, lifecycle, labels, agent boundaries, recovery. |
| `.editorconfig`, `.gitattributes`, `.gitignore` | Baseline hygiene. |

## Starting a project

1. Create the repo from this template — GitHub's **Use this template**, or
   `gh repo create <name> --template kevindurb/base --private --clone`.
2. `just install-hooks` — once per clone.
3. Fill in the contract in `justfile`: replace the failing stubs for `fmt`,
   `lint`, `build`, and `test` with the project's real commands. `fmt` and
   `lint` must check, not fix.
4. Fill in `AGENTS.md`: purpose, the commands table, and conventions.
5. Fill in the `context:` block in `openspec/config.yaml` — it is what agents
   read before writing a proposal.
6. In `justfile`, change `ci: check-template` to
   `ci: pre-commit pre-push spec-validate` and delete the `check-template`
   recipe.
7. Set up the factory, following `docs/factory.md`:
   - run `scripts/factory-labels.sh --repo OWNER/NAME`;
   - create a Tailscale federated identity for GitHub Actions WIF and add the
     `TS_CLIENT_ID` repository variable (and `TS_AUDIENCE` only for a custom
     audience); use the `sub` prefix printed by the smoke workflow as the
     identity subject; do not create `TS_OAUTH_SECRET`;
   - add the tailnet policy grant for `tag:ci` to `tag:ai` on `tcp:443` with the
     Aperture `role: user` and model capability (and the read-only MCP web
     connectors if used);
   - protect `main` with a required PR and the `CI` check;
   - allow GitHub Actions to create pull requests;
   - run the **OpenSpec smoke** workflow to verify the OIDC exchange and model
     preflight.
   The factory is opt-in for an existing repository; a new repository from the
   template inherits the files but still needs these settings.
8. Rewrite this README and add project docs under `docs/`.
9. Commit.

### About this repo itself

`base` has no code, so its own `ci` points at `just check-template`, which
verifies the scaffolding is intact. CI additionally installs OpenSpec and runs
`openspec validate --all`, so factory artifacts are validated here even though
the full lifecycle must be tested in a derived repository.

Do **not** run `just install-hooks` here: the stubs are meant to fail and would
block template edits.

## Deliberately not included

Stack-specific build, lint, and test commands; release and tagging policy;
deploy steps; a licence; `CLAUDE.md` / `.claude/`; and stack overlay
directories. Those are first-commit decisions for each project, not template
defaults.

The factory also does not include auto-merge, autonomous issue triage, a
quick-fix lane that bypasses OpenSpec, multi-repo orchestration, or a GitHub
App for bot-driven label transitions. Those are deliberate v1 boundaries, not
omissions to patch around.
