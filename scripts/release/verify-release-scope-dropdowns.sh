#!/usr/bin/env bash
# scripts/release/verify-release-scope-dropdowns.sh
#
# Verifies that the hand-maintained `workflow_dispatch` `scope` choice
# dropdowns in the release workflows match the authoritative set of release
# scopes declared in scripts/release/release.config.json (`.scopes` keys).
#
# Why this matters: the release workflows expose a `scope` input as a
# `type: choice` with a hard-coded `options:` list. That list is supposed to
# be "regenerated from release.config.json", but nothing enforced it — so as
# packages were enrolled/renamed in release.config.json the dropdowns drifted
# (newly-enrolled packages weren't canary-selectable; stale scopes lingered).
# This guard fails CI whenever a dropdown diverges from the config.
#
# Two files are checked:
#   .github/workflows/publish-release.yml  — canary/prerelease `scope` input
#   .github/workflows/prepare-release.yml  — create-pr `scope` input
#
# Sentinel exception: neither workflow uses a non-scope sentinel option (no
# `all` / `canary` pseudo-scope — an empty/omitted scope is handled outside
# the options list). If a sentinel is ever introduced, add it to
# SENTINELS below so it is excluded from the equality check.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CONFIG="$REPO_ROOT/scripts/release/release.config.json"
PUBLISH_WF="$REPO_ROOT/.github/workflows/publish-release.yml"
PREPARE_WF="$REPO_ROOT/.github/workflows/prepare-release.yml"

# Documented non-scope sentinel options to ignore (none today). Space-separated.
SENTINELS=""

for f in "$CONFIG" "$PUBLISH_WF" "$PREPARE_WF"; do
  if [ ! -f "$f" ]; then
    echo "ERROR: $f not found" >&2
    exit 1
  fi
done

# Authoritative scope set from release.config.json.
CONFIG_SCOPES=$(jq -r '.scopes | keys[]' "$CONFIG" | sort -u)

# Extract the `options:` list belonging to the `scope:` input from a workflow.
# Uses yq when available, otherwise a robust awk pass:
#   - find the `scope:` input key (an `inputs:` child, indented 6 spaces),
#   - within that block find its `options:` line,
#   - collect the `- value` list items until indentation drops back out.
extract_scope_options() {
  local file="$1"
  if command -v yq >/dev/null 2>&1; then
    yq -r '.on.workflow_dispatch.inputs.scope.options[]' "$file" | sort -u
    return
  fi
  awk '
    # Match the scope input key: "      scope:" (6-space indent under inputs:).
    /^      scope:[[:space:]]*$/ { in_scope = 1; next }
    in_scope && /^      [a-zA-Z0-9_-]+:[[:space:]]*$/ { in_scope = 0 }   # next sibling input
    in_scope && /^        options:[[:space:]]*$/ { in_opts = 1; next }
    in_opts {
      # An options list item: "          - value"
      if (match($0, /^[[:space:]]*-[[:space:]]+/)) {
        val = $0
        sub(/^[[:space:]]*-[[:space:]]+/, "", val)
        sub(/[[:space:]]+$/, "", val)
        print val
        next
      }
      # Any non-list-item line ends the options block.
      in_opts = 0
      in_scope = 0
    }
  ' "$file" | sort -u
}

# Strip documented sentinels from an option set before comparing.
strip_sentinels() {
  local opts="$1"
  if [ -z "$SENTINELS" ]; then
    printf '%s\n' "$opts"
    return
  fi
  local filtered="$opts"
  for s in $SENTINELS; do
    filtered=$(printf '%s\n' "$filtered" | grep -vx "$s" || true)
  done
  printf '%s\n' "$filtered"
}

check_workflow() {
  local name="$1" file="$2"
  local opts
  opts=$(extract_scope_options "$file")
  opts=$(strip_sentinels "$opts")

  if [ -z "$opts" ]; then
    echo "ERROR: could not extract any scope options from $name ($file)" >&2
    return 1
  fi

  if [ "$opts" = "$CONFIG_SCOPES" ]; then
    echo "OK: $name scope dropdown matches release.config.json scopes"
    return 0
  fi

  echo "ERROR: $name scope dropdown is out of sync with release.config.json." >&2
  echo "" >&2
  echo "--- diff (release.config.json scopes  vs  $name options) ---" >&2
  diff <(printf '%s\n' "$CONFIG_SCOPES") <(printf '%s\n' "$opts") >&2 || true
  echo "" >&2
  echo "Fix: update the 'scope' input 'options:' list in $file to exactly match" >&2
  echo "the keys of '.scopes' in scripts/release/release.config.json" >&2
  echo "(plus any documented sentinel listed in SENTINELS within this script)." >&2
  return 1
}

rc=0
check_workflow "publish-release.yml" "$PUBLISH_WF" || rc=1
check_workflow "prepare-release.yml" "$PREPARE_WF" || rc=1

if [ "$rc" -ne 0 ]; then
  exit 1
fi

echo "OK: both release scope dropdowns match release.config.json"
exit 0
