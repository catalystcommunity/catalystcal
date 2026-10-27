#!/usr/bin/env bash
# Typecheck, unit-test, and build the SPA (webapp/).
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$SRC_DIR/webapp"
install_node

log "npm ci"
npm ci
log "typecheck"
npm run typecheck
log "test"
npm test
log "build"
npm run build
log "Web checks passed"
