#!/usr/bin/env bash
# Regenerate everything driven by catalystcal.csil. Run from any directory.
#
#   Go server  → api/internal/csil/    (consumed by the api server)
#   Clients    → clients/<lang>/        (one package per language)
#
# The Go server target emits `package api`; we rewrite it to `package csil` so
# the import path's last segment matches the package name.
#
# The SPA (webapp/) no longer carries an in-tree generated client: it consumes
# clients/typescript (@catalystcal/client). All client generation lives in
# clients/regenerate.sh, which this script delegates to after the server.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="$REPO/csil/catalystcal.csil"
GO_OUT="$REPO/api/internal/csil"

# csilgen is installed via `cargo install` and lives at ~/.cargo/bin/csilgen.
# That directory isn't always on the non-interactive PATH (e.g. shells spawned
# by editors/tools that don't source the user rc), so add it here rather than
# forcing every caller to fix their environment.
if [ -d "$HOME/.cargo/bin" ]; then
    export PATH="$HOME/.cargo/bin:$PATH"
fi

if ! command -v csilgen >/dev/null; then
    echo "ERROR: csilgen not on PATH (looked in \$PATH and ~/.cargo/bin)" >&2
    echo "       install with: cargo install --git https://github.com/catalystcommunity/csilgen" >&2
    exit 1
fi

# The pinned csilgen release. A different CLI (or its generators) changes the
# generated code silently, so stop here instead.
CSILGEN_PIN="0.2.10"
if [ "$(csilgen --version 2>/dev/null)" != "csilgen $CSILGEN_PIN" ]; then
    echo "ERROR: need csilgen $CSILGEN_PIN, found: $(csilgen --version 2>&1)" >&2
    exit 1
fi

csilgen validate --input "$SPEC"

# ---- Go server (api) ----
# Bare `go` target = the server surface (types + service interfaces). No
# emit_packages here: this is in-tree server code, not a standalone package.
mkdir -p "$GO_OUT"
csilgen generate --input "$SPEC" --target go --output "$GO_OUT"
for f in "$GO_OUT"/*.gen.go; do
    sed -i 's/^package api$/package csil/' "$f"
done
if command -v gofmt >/dev/null; then
    gofmt -w "$GO_OUT"
fi

# ---- Clients (all languages) ----
# Delegates to clients/regenerate.sh, which emits a self-contained package per
# language into clients/<lang>/. Kept separate because client generation needs
# per-language emit_packages/coordinates that must NOT leak into the server gen
# above (which shares this spec).
"$REPO/clients/regenerate.sh"

echo
echo "Regenerated Go server: $(ls "$GO_OUT"/*.gen.go | wc -l) file(s) in $GO_OUT"
