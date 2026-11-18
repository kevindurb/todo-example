#!/usr/bin/env bash
# Derive the change name from an issue number and title.
#
# The name is the factory's only durable mapping between an issue and its
# OpenSpec change: `gh<number>-<kebab-case-slug>`. Keep it lowercase, ASCII,
# and short enough for a branch.
set -euo pipefail

if (( $# < 2 )); then
  printf 'usage: %s ISSUE_NUMBER ISSUE_TITLE...\n' "$0" >&2
  exit 2
fi

issue=$1
shift
title=$*

[[ $issue =~ ^[0-9]+$ ]] || { printf 'issue number must be numeric (got %s)\n' "$issue" >&2; exit 2; }

slug=$(
  printf '%s' "$title" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -e 's/[^a-z0-9]\+/-/g' -e 's/^-//' -e 's/-$//' \
    | cut -c1-48 \
    | sed -e 's/-$//'
)

[[ -n $slug ]] || slug=change
printf 'gh%s-%s\n' "$issue" "$slug"
