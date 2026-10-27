#!/usr/bin/env bash
# Check that each commit in the pull request has a conventional commit subject:
#   type(scope)?!?: description
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$SRC_DIR"

base="${REACTORCIDE_DIFF_BASE:-}"
if [ -z "$base" ]; then
    branch="${REACTORCIDE_BASE_REF:-main}"
    for ref in "origin/$branch" "$branch"; do
        if base="$(git merge-base HEAD "$ref" 2>/dev/null)"; then
            break
        fi
        base=""
    done
fi
range="HEAD"
[ -n "$base" ] && range="${base}..HEAD"
log "Check commit subjects in ${range}"

pattern='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([^)]+\))?!?: .+'
fail=0
while IFS= read -r line; do
    sha="${line%% *}"
    subject="${line#* }"
    if [[ "$subject" =~ $pattern ]]; then
        echo "OK:   $subject"
    else
        echo "FAIL: $subject ($sha)"
        fail=1
    fi
done < <(git log --no-merges --format='%h %s' "$range")

if [ "$fail" != 0 ]; then
    die "commit subjects must match 'type(scope)?: description'. Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert."
fi
log "All commit subjects are conventional"
