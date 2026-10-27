#!/usr/bin/env bash
# Build one image (CATALYSTCAL_IMAGE=api|web) without a push.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$SRC_DIR"

case "${CATALYSTCAL_IMAGE:-}" in
    api) dockerfile=Dockerfile.api ;;
    web) dockerfile=Dockerfile.web ;;
    *) die "CATALYSTCAL_IMAGE must be api or web" ;;
esac

wait_for_builder
log "Build ${dockerfile} (no push)"
build_image "$dockerfile" "$(tr -d '[:space:]' < version/VERSION.txt)-ci"
log "${dockerfile} builds"
