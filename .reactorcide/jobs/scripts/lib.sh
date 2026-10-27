#!/usr/bin/env bash
# Shared helpers for the catalystcal CI scripts. Source it; do not run it.
#
# Every tool that the runner image does not carry is downloaded at a pinned
# version and checked against a pinned SHA-256 (or the digest GitHub records
# for a release asset). The runner image has go, git, curl. It has no node,
# helm, docker, buildctl, crane, gh, or csilgen.

set -euo pipefail

SRC_DIR="${REACTORCIDE_CODE_DIR:-/job/src}"

# Jobs can run without root (pr-untrusted profiles). Keep every cache under
# $HOME and never call sudo.
export HOME="${HOME:-/tmp/home}"
mkdir -p "$HOME"
LOCAL_BIN="$HOME/.local/bin"
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

NODE_VERSION="24.21.0"
NODE_SHA256_X64="fd8e59d5a511510f6a298afb548f18c7d2b1be404d8b4a27d94fbe49f56cb2d6"
HELM_VERSION="3.19.0"
HELM_SHA256_AMD64="a7f81ce08007091b86d8bd696eb4d86b8d0f2e1b9f6c714be62f82f96a594496"
BUILDKIT_VERSION="0.17.3"
DOCKER_CLI_VERSION="27.5.1"
CRANE_VERSION="0.20.3"

log() { echo "=== $* ==="; }
die() { echo "ERROR: $*" >&2; exit 1; }

# fetch_verified URL DEST SHA256: download, then check the digest.
fetch_verified() {
    local url="$1" dest="$2" want="$3" got
    curl -fsSL --retry 3 "$url" -o "$dest"
    got="$(sha256sum "$dest" | cut -d' ' -f1)"
    if [ "$got" != "$want" ]; then
        rm -f "$dest"
        die "$url: SHA-256 $got does not match $want"
    fi
}

setup_go() {
    export GOPATH="$HOME/go"
    export GOMODCACHE="$GOPATH/pkg/mod"
    export GOCACHE="$HOME/.cache/go-build"
    export GOFLAGS="${GOFLAGS:-} -mod=readonly"
    mkdir -p "$GOMODCACHE" "$GOCACHE"
    go version
}

install_node() {
    if command -v node >/dev/null 2>&1 && [ "$(node --version)" = "v${NODE_VERSION}" ]; then
        return
    fi
    [ "$(uname -m)" = "x86_64" ] || die "pinned Node digest is for x86_64 only"
    log "Install Node ${NODE_VERSION}"
    local name="node-v${NODE_VERSION}-linux-x64"
    fetch_verified "https://nodejs.org/dist/v${NODE_VERSION}/${name}.tar.xz" \
        "/tmp/${name}.tar.xz" "$NODE_SHA256_X64"
    rm -rf "$HOME/.local/${name}"
    tar -xJf "/tmp/${name}.tar.xz" -C "$HOME/.local"
    rm -f "/tmp/${name}.tar.xz"
    export PATH="$HOME/.local/${name}/bin:$PATH"
    export npm_config_cache="$HOME/.npm"
    node --version
}

install_helm() {
    if command -v helm >/dev/null 2>&1; then
        return
    fi
    log "Install helm ${HELM_VERSION}"
    local archive="/tmp/helm-v${HELM_VERSION}-linux-amd64.tar.gz"
    fetch_verified "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" \
        "$archive" "$HELM_SHA256_AMD64"
    tar -xzf "$archive" -C /tmp linux-amd64/helm
    mv /tmp/linux-amd64/helm "$LOCAL_BIN/helm"
    rm -rf "$archive" /tmp/linux-amd64
    helm version --short
}

# install_csilgen ROOT: install the csilgen release that csil/regenerate.sh
# pins (CSILGEN_PIN) under ROOT, with its generators in ROOT/home. Do not use
# csilgen's tools.sh install-all: it installs the NEWEST release, not the pin.
# Each asset is checked against the SHA-256 digest that GitHub records.
# Exports HOME and PATH so only this install is used.
install_csilgen() {
    local root="$1" version api assets
    version="$(sed -n 's/^CSILGEN_PIN="\([0-9.]*\)"$/\1/p' "$SRC_DIR/csil/regenerate.sh")"
    [ -n "$version" ] || die "csil/regenerate.sh does not set CSILGEN_PIN"
    log "Install csilgen ${version}"
    mkdir -p "$root/bin" "$root/home/.csilgen/generators" "$root/dl"
    api="https://api.github.com/repos/catalystcommunity/csilgen/releases/tags/csilgen%2Fv${version}"
    assets="$root/dl/release.json"
    curl -fsSL --retry 3 ${GITHUB_TOKEN:+-H "Authorization: Bearer ${GITHUB_TOKEN}"} "$api" -o "$assets"

    fetch_asset() {
        local name="$1" url digest
        read -r url digest < <(python3 -c '
import json, sys
info = json.load(open(sys.argv[1]))
for a in info.get("assets", []):
    if a["name"] == sys.argv[2]:
        d = a.get("digest") or ""
        if not d.startswith("sha256:"):
            sys.exit("asset has no sha256 digest: " + sys.argv[2])
        print(a["browser_download_url"], d.split(":", 1)[1])
        break
else:
    sys.exit("release has no asset " + sys.argv[2])
' "$assets" "$name")
        fetch_verified "$url" "$root/dl/$name" "$digest"
    }

    fetch_asset "csilgen-${version}-x86_64-unknown-linux-gnu.tar.gz"
    tar -xzf "$root/dl/csilgen-${version}-x86_64-unknown-linux-gnu.tar.gz" -C "$root/bin" \
        --no-anchored --transform='s|^\./||' csilgen
    chmod 0755 "$root/bin/csilgen"

    fetch_asset "csilgen-generators-${version}.tar.gz"
    tar -xzf "$root/dl/csilgen-generators-${version}.tar.gz" \
        -C "$root/home/.csilgen/generators" --wildcards --no-anchored '*.wasm'
    ls "$root/home/.csilgen/generators/"*.wasm >/dev/null 2>&1 \
        || die "csilgen ${version} generator bundle has no .wasm files"

    export HOME="$root/home"
    export PATH="$root/bin:$PATH"
    local reported
    reported="$(csilgen --version)"
    [ "$reported" = "csilgen ${version}" ] || die "installed csilgen reports '$reported', not ${version}"
    log "Installed ${reported}"
}

# wait_for_builder: CI image builds use a docker daemon when DOCKER_HOST is
# set, otherwise the buildkit sidecar (builder capability).
wait_for_builder() {
    if [ -n "${DOCKER_HOST:-}" ]; then
        if ! command -v docker >/dev/null 2>&1; then
            log "Install docker CLI ${DOCKER_CLI_VERSION}"
            curl -fsSL "https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_CLI_VERSION}.tgz" -o /tmp/docker.tgz
            tar -xzf /tmp/docker.tgz --strip-components=1 -C "$LOCAL_BIN" docker/docker
            rm -f /tmp/docker.tgz
        fi
        local i
        for i in $(seq 1 30); do
            docker info >/dev/null 2>&1 && return
            sleep 1
        done
        die "docker daemon not ready after 30 seconds"
    fi
    if ! command -v buildctl >/dev/null 2>&1; then
        log "Install buildctl ${BUILDKIT_VERSION}"
        curl -fsSL "https://github.com/moby/buildkit/releases/download/v${BUILDKIT_VERSION}/buildkit-v${BUILDKIT_VERSION}.linux-amd64.tar.gz" -o /tmp/buildkit.tar.gz
        tar -xzf /tmp/buildkit.tar.gz --strip-components=1 -C "$LOCAL_BIN" bin/buildctl
        rm -f /tmp/buildkit.tar.gz
    fi
    local i
    for i in $(seq 1 30); do
        buildctl debug info >/dev/null 2>&1 && return
        sleep 1
    done
    die "builder sidecar not ready after 30 seconds"
}

# build_image DOCKERFILE VERSION [OUTPUT...]: OUTPUT is a list of image refs
# to push. With no refs, build only (PR check).
build_image() {
    local dockerfile="$1" version="$2"
    shift 2
    if [ -n "${DOCKER_HOST:-}" ]; then
        local tag="catalystcal-ci:${version}"
        docker build -f "$dockerfile" --build-arg "VERSION=${version}" -t "$tag" .
        if [ $# -gt 0 ]; then
            install_crane
            docker save "$tag" -o /tmp/image.tar
            local ref
            for ref in "$@"; do
                crane push /tmp/image.tar "$ref"
            done
            rm -f /tmp/image.tar
        fi
        return
    fi
    if [ $# -eq 0 ]; then
        buildctl build --frontend dockerfile.v0 --local context=. --local dockerfile=. \
            --opt "filename=${dockerfile}" --opt "build-arg:VERSION=${version}" \
            --output "type=oci,dest=/tmp/catalystcal-image.tar"
        rm -f /tmp/catalystcal-image.tar
        return
    fi
    local names
    names="$(IFS=,; echo "$*")"
    buildctl build --frontend dockerfile.v0 --local context=. --local dockerfile=. \
        --opt "filename=${dockerfile}" --opt "build-arg:VERSION=${version}" \
        --output "type=image,\"name=${names}\",push=true"
}

install_crane() {
    command -v crane >/dev/null 2>&1 && return
    log "Install crane ${CRANE_VERSION}"
    curl -fsSL "https://github.com/google/go-containerregistry/releases/download/v${CRANE_VERSION}/go-containerregistry_Linux_x86_64.tar.gz" -o /tmp/crane.tar.gz
    tar -xzf /tmp/crane.tar.gz -C "$LOCAL_BIN" crane
    rm -f /tmp/crane.tar.gz
}

# registry_login: write ~/.docker/config.json for crane and buildctl.
registry_login() {
    local registry="$1"
    [ -n "${REGISTRY_USER:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ] || die "REGISTRY_USER and REGISTRY_PASSWORD must be set"
    mkdir -p "$HOME/.docker"
    local auth
    auth="$(printf '%s:%s' "$REGISTRY_USER" "$REGISTRY_PASSWORD" | base64 -w 0)"
    umask 077
    printf '{"auths":{"%s":{"auth":"%s"}}}\n' "$registry" "$auth" > "$HOME/.docker/config.json"
}
