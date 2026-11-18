#!/usr/bin/env bash
# shellcheck disable=SC2016  # Backticks below are literal Markdown.
# Create or update the single factory status comment on an issue.
#
# REPO, ISSUE_NUMBER, CHANGE_NAME, STATE, and NEXT_ACTION are required. The
# three PR numbers are optional; each is resolved to a link and state when it
# exists. The marker at the top makes the comment idempotent.
set -euo pipefail

repo=${REPO:?REPO is required}
issue=${ISSUE_NUMBER:?ISSUE_NUMBER is required}
change=${CHANGE_NAME:?CHANGE_NAME is required}
state=${STATE:?STATE is required}
next=${NEXT_ACTION:-}
proposal=${PROPOSAL_PR:-}
impl=${IMPL_PR:-}
archive=${ARCHIVE_PR:-}
blocked=${BLOCKED_REASON:-}
run=${RUN_URL:-}

marker='<!-- openspec-factory-status -->'

pr_link() {
  local number=$1 meta state url
  if [[ -z $number ]]; then
    printf '—'
    return
  fi
  meta=$(gh pr view "$number" --repo "$repo" --json url,state --jq '"\(.state)|\(.url)"' 2>/dev/null || true)
  if [[ -z $meta ]]; then
    printf '#%s' "$number"
    return
  fi
  state=${meta%%|*}
  url=${meta#*|}
  printf '[#%s](%s) (%s)' "$number" "$url" "$(printf '%s' "$state" | tr '[:upper:]' '[:lower:]')"
}

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

{
  printf '%s\n' "$marker"
  printf '## OpenSpec factory status\n\n'
  printf -- '- **Change:** `%s`\n' "$change"
  printf -- '- **State:** `%s`\n' "$state"
  if [[ -n $blocked ]]; then
    printf -- '- **Blocked:** %s\n' "$blocked"
  fi
  printf '\n'
  printf '| Phase | Pull request |\n'
  printf '| --- | --- |\n'
  printf '| Proposal | %s |\n' "$(pr_link "$proposal")"
  printf '| Implementation | %s |\n' "$(pr_link "$impl")"
  printf '| Archive | %s |\n\n' "$(pr_link "$archive")"
  printf '**Next human action:** %s\n' "${next:-None.}"
  if [[ -n $run ]]; then
    printf '\n[Workflow run](%s)\n' "$run"
  fi
} > "$tmp"

existing=$(
  gh api --paginate --slurp "repos/$repo/issues/$issue/comments" \
    | jq -r --arg marker "$marker" '[.[][] | select(.body | startswith($marker)) | .id] | first // empty'
)

if [[ -n $existing ]]; then
  jq -n --arg body "$(cat "$tmp")" '{body: $body}' \
    | gh api -X PATCH "repos/$repo/issues/comments/$existing" --input - >/dev/null
  printf 'factory-status: updated comment %s on #%s\n' "$existing" "$issue"
else
  gh issue comment "$issue" --repo "$repo" --body-file "$tmp" >/dev/null
  printf 'factory-status: created status comment on #%s\n' "$issue"
fi
