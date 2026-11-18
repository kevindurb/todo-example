#!/usr/bin/env bash
# Validate one agent's result contract and expose it to the workflow.
#
# Required fields: status, phase, change, summary. A blocked result requires
# blocked_reason; a successful verify requires verdict; an ok result requires a
# non-empty report. Missing or invalid input exits 2 so the caller can take its
# failure path instead of opening a PR.
set -euo pipefail

phase=${EXPECTED_PHASE:?EXPECTED_PHASE is required}
change=${EXPECTED_CHANGE:?EXPECTED_CHANGE is required}
result=${RESULT_FILE:-.agent/result.json}
report=${REPORT_FILE:-.agent/report.md}

fail() {
  printf 'factory-result: %s\n' "$*" >&2
  exit 2
}

[[ -f $result ]] || fail "missing $result"
jq -e 'type == "object"' "$result" >/dev/null 2>&1 || fail "$result is not a JSON object"

status=$(jq -r '.status // empty' "$result")
phase_value=$(jq -r '.phase // empty' "$result")
change_value=$(jq -r '.change // empty' "$result")
summary=$(jq -r '.summary // empty' "$result")
blocked_reason=$(jq -r '.blocked_reason // empty' "$result")
verdict=$(jq -r '.verdict // empty' "$result")

case "$status" in
  ok | blocked | failed) ;;
  *) fail "status must be one of ok, blocked, failed (got '${status}')" ;;
esac

[[ $phase_value == "$phase" ]] || fail "phase must be '$phase' (got '${phase_value}')"
[[ $change_value == "$change" ]] || fail "change must be '$change' (got '${change_value}')"
[[ -n $summary ]] || fail "summary is required"

if [[ $status == blocked && -z $blocked_reason ]]; then
  fail "blocked_reason is required when status is blocked"
fi

if [[ $phase == verify && $status == ok ]]; then
  case "$verdict" in
    pass | concerns) ;;
    *) fail "verdict must be pass or concerns for a successful verify (got '${verdict}')" ;;
  esac
fi

if [[ $status == ok ]]; then
  [[ -s $report ]] || fail "missing or empty $report"
fi

if [[ -n ${GITHUB_OUTPUT:-} ]]; then
  delimiter="FACTORY_$(date +%s%N)_$$"
  {
    printf 'status=%s\n' "$status"
    printf 'phase=%s\n' "$phase_value"
    printf 'change=%s\n' "$change_value"
    printf 'verdict=%s\n' "$verdict"
    printf 'summary<<%s\n%s\n%s\n' "$delimiter" "$summary" "$delimiter"
    printf 'blocked_reason<<%s\n%s\n%s\n' "$delimiter" "$blocked_reason" "$delimiter"
  } >> "$GITHUB_OUTPUT"
fi

printf 'factory-result: %s (%s)\n' "$status" "$summary"
