#!/usr/bin/env bash
# Build one release image (CATALYSTCAL_IMAGE=api|web) and push it to
#   ${REGISTRY}/${REGISTRY_PATH}/<api|web>:<version> and :latest
# Runs on a v* tag. The version comes from the tag name.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$SRC_DIR"

case "${CATALYSTCAL_IMAGE:-}" in
    api) dockerfile=Dockerfile.api ;;
    web) dockerfile=Dockerfile.web ;;
    *) die "CATALYSTCAL_IMAGE must be api or web" ;;
esac

if [[ "${REACTORCIDE_EVENT_TYPE:-}" = "tag_created" && "${REACTORCIDE_BRANCH:-}" = v* ]]; then
    version="${REACTORCIDE_BRANCH#v}"
else
    version="$(tr -d '[:space:]' < version/VERSION.txt)"
fi
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "not a release version: '$version'"

# The tag points at the merge commit, before the release job commits the
# version bump. The SPA reads version/VERSION.txt at build time, so write the
# tag version into this throwaway checkout first.
echo "$version" > version/VERSION.txt

registry="${REGISTRY:-containers.catalystsquad.com}"
repo="${registry}/${REGISTRY_PATH:-public/catalystcal}/${CATALYSTCAL_IMAGE}"

registry_login "$registry"
wait_for_builder
log "Build and push ${repo}:${version}"
build_image "$dockerfile" "$version" "${repo}:${version}" "${repo}:latest"
log "Pushed ${repo}:${version} and ${repo}:latest"
