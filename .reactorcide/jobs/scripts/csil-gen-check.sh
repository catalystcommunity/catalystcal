#!/usr/bin/env bash
# Regenerate all CSIL output with the pinned csilgen release, then fail if the
# committed generated code differs. New untracked files also fail: csilgen
# can add a file (for example the <spec>.csil-schema.cbor descriptor).
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$SRC_DIR"
setup_go

scratch="$(mktemp -d /tmp/csilgen.XXXXXX)"
trap 'rm -rf "$scratch"' EXIT
real_home="$HOME"
install_csilgen "$scratch"

bash csil/regenerate.sh
export HOME="$real_home"

paths=(api/internal/csil clients)
status="$(git status --porcelain -- "${paths[@]}")"
if [ -n "$status" ]; then
    echo "$status"
    git --no-pager diff --stat -- "${paths[@]}" || true
    die "generated CSIL code is stale. Run './tools gen' with the pinned csilgen and commit the result, including new files."
fi
log "Generated code matches csil/catalystcal.csil"
