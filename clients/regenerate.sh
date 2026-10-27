#!/usr/bin/env bash
# Regenerate the CatalystCal client libraries for every language csilgen can
# package, into clients/<lang>/. Each output dir is a self-contained,
# dependency-free package (manifest + barrel + types + codec + client) — the
# generated codec owns the wire, so there is no csilgen-transport dependency to
# vendor or pin. Run from any directory.
#
#   clients/<lang>/    one publishable client package per language
#
# csilgen is treated as an ordinary system tool (like docker/gofmt): it must be
# on PATH. Install it from the repo if missing — see the error below.
#
# Coordinates (package name, module path, version) are per-language because each
# ecosystem has its own name grammar; we splice a language-specific options block
# onto the shared spec for each generation rather than committing 14 option sets
# into catalystcal.csil (which is also consumed by the api server generation, and
# must stay free of emit_packages — see csil/regenerate.sh).
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="$REPO/csil/catalystcal.csil"
OUT="$REPO/clients"
VERSION="$(tr -d '[:space:]' < "$REPO/version/VERSION.txt")"
GO_MODULE_BASE="github.com/catalystcommunity/catalystcal/clients"

# csilgen is installed via `cargo install` and usually lives in ~/.cargo/bin,
# which isn't always on a non-interactive PATH. Add it before looking.
if [ -d "$HOME/.cargo/bin" ]; then
    export PATH="$HOME/.cargo/bin:$PATH"
fi

if ! command -v csilgen >/dev/null; then
    echo "ERROR: csilgen not on PATH (looked in \$PATH and ~/.cargo/bin)" >&2
    echo "       install it from the repo:" >&2
    echo "         cargo install --git https://github.com/catalystcommunity/csilgen" >&2
    echo "       (then: cargo run -p xtask install-wasm, per its README)" >&2
    exit 1
fi

csilgen validate --input "$SPEC"

# Every language csilgen can emit a package for. C and Zig package too (no
# central registry, but a buildable source package + quickstart). Format:
#   <lang> <target> <extra-options>
# <target> is the csilgen target; the *-client subtargets (go/rust/python/
# typescript) emit a client-only package, the bare targets emit all surfaces.
# <extra-options> are spliced into the options block verbatim (comma-joined).
LANGS=(
  "typescript|typescript-client|package_name: \"@catalystcal/client\", import_extension: \"none\""
  "go|go-client|package_name: \"$GO_MODULE_BASE/go\", go_module: \"$GO_MODULE_BASE/go\", go_package: \"catalystcalclient\""
  "rust|rust-client|package_name: \"catalystcal-client\""
  "python|python-client|package_name: \"catalystcal_client\""
  "ruby|ruby|package_name: \"catalystcal_client\""
  "elixir|elixir|package_name: \"catalystcal_client\""
  "java|java|package_name: \"catalystcal-client\", java_package: \"com.catalystcommunity.catalystcal.client\""
  "kotlin|kotlin|package_name: \"catalystcal-client\", kotlin_package: \"com.catalystcommunity.catalystcal.client\""
  "csharp|csharp|package_name: \"CatalystCal.Client\""
  "ocaml|ocaml|package_name: \"catalystcal_client\""
  "swift|swift|package_name: \"CatalystCalClient\""
  "dart|dart|package_name: \"catalystcal_client\""
  "c|c|package_name: \"catalystcal_client\""
  "zig|zig|package_name: \"catalystcal_client\""
)

# build_spec writes a temp spec whose options block carries emit_packages +
# coordinates for one language; the rest of the spec is untouched. The original
# leading `options { ... }` block (lines up to the first closing brace) is
# dropped and replaced.
build_spec() {
    local lang="$1" extra="$2" out="$3"
    {
        echo "options {"
        echo "    package: \"catalystcal\","
        echo "    version: \"v1alpha\","
        echo "    package_version: \"$VERSION\","
        echo "    emit_packages: [\"$lang\"],"
        echo "    $extra"
        echo "}"
        awk 'skip && /^}/ {skip=0; next} NR==1 && /^options[[:space:]]*\{/ {skip=1; next} !skip' "$SPEC"
    } > "$out"
}

# Generators known to fail on the catalystcal spec, tracked as expected failures so
# a clean run stays green (a non-empty entry should reference a csilgen request in
# csilgen/docs/csilgen-requests/). Empty = every generator is expected to work.
KNOWN_BROKEN=""

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

ok=()
fail=()
for entry in "${LANGS[@]}"; do
    IFS="|" read -r lang target extra <<<"$entry"
    spec="$TMPDIR/catalystcal-$lang.csil"
    dest="$OUT/$lang"
    build_spec "$lang" "$extra" "$spec"
    rm -rf "$dest"
    mkdir -p "$dest"
    # csilgen currently exits 0 even when a generator panics mid-run (the WASM
    # trap is logged, not propagated as a non-zero status), so success is gated
    # on the generator having actually emitted files, not on the exit code.
    csilgen generate --input "$spec" --target "$target" --output "$dest" --quiet \
        2>"$TMPDIR/$lang.err" || true
    if [ "$(find "$dest" -type f | wc -l | tr -d ' ')" -gt 0 ]; then
        ok+=("$lang")
    else
        fail+=("$lang")
        rmdir "$dest" 2>/dev/null || true
        echo "  ! $lang ($target) emitted no files:" >&2
        sed 's/^/      /' "$TMPDIR/$lang.err" >&2
    fi
done

# Go: gofmt the generated package so it matches repo style and is verifiably
# parseable.
if command -v gofmt >/dev/null && [ -d "$OUT/go" ]; then
    gofmt -w "$OUT/go" 2>/dev/null || true
fi

echo
echo "Regenerated client packages:"
for lang in "${ok[@]}"; do
    n="$(find "$OUT/$lang" -type f | wc -l | tr -d ' ')"
    echo "  ok       $lang  ($n files)"
done
unexpected=0
for lang in "${fail[@]}"; do
    if [[ " $KNOWN_BROKEN " == *" $lang "* ]]; then
        echo "  skip     $lang  (known csilgen generator crash — tracked upstream)"
    else
        echo "  FAIL     $lang  (unexpected — a previously-working generator broke)"
        unexpected=1
    fi
done
exit $unexpected
