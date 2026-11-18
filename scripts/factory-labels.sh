#!/usr/bin/env bash
# Create or update every label the factory uses.
#
# Run once in a repository created from this template, and again after a label
# change. `gh label create --force` updates an existing label in place.
set -euo pipefail

repo_args=()
if [[ ${1:-} == --repo ]]; then
  [[ -n ${2:-} ]] || { printf 'usage: %s [--repo OWNER/NAME]\n' "$0" >&2; exit 2; }
  repo_args=(--repo "$2")
elif (( $# > 0 )); then
  printf 'usage: %s [--repo OWNER/NAME]\n' "$0" >&2
  exit 2
fi

labels=(
  # Command labels start a phase. A maintainer applies one; the phase removes it.
  'opsx:propose|1d76db|Start the propose phase for this issue'
  'opsx:implement|0e8a16|Start the implementation phase for the approved change'
  'opsx:archive|5319e7|Archive the implemented change and sync its specs'

  # Running overlays are additive; the lifecycle label stays visible beside them.
  'opsx:proposing|fbca04|A propose run is in progress'
  'opsx:implementing|fbca04|An implementation run is in progress'
  'opsx:archiving|fbca04|An archive run is in progress'
  'opsx:verifying|fbca04|An advisory verification run is in progress'
  'opsx:revising|fbca04|A revise run is in progress'

  # Lifecycle labels record the gate the change is waiting on.
  'opsx:proposal-review|c2e0c6|Proposal PR is open for spec review'
  'opsx:proposal-approved|0e8a16|Proposal merged; apply opsx:implement to build'
  'opsx:impl-review|c2e0c6|Implementation PR is open for code review'
  'opsx:impl-approved|0e8a16|Implementation merged; apply opsx:archive to finish'
  'opsx:archive-review|c2e0c6|Archive PR is open for review'
  'opsx:done|6f42c1|The change is archived and the issue is complete'

  # Failure overlay. Remove it before retrying the same command label.
  'opsx:blocked|b60205|The last phase stopped; read the reason before retrying'

  # Triage label used by the issue form until a maintainer accepts the request.
  'opsx:triage|d4c5f9|Needs maintainer triage before a factory command'
)

for entry in "${labels[@]}"; do
  IFS='|' read -r name color description <<< "$entry"
  gh label create "$name" --color "$color" --description "$description" --force "${repo_args[@]}" >/dev/null
  printf 'label: %s\n' "$name"
done

printf 'factory-labels: %d labels configured\n' "${#labels[@]}"
