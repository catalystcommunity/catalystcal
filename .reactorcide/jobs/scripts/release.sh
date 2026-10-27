#!/usr/bin/env bash
# Release on merge to main:
#   1. semver-tags reads the conventional commits since the last v* tag,
#      decides the bump, and pushes the new tag (vX.Y.Z). The tag starts the
#      catalystcal-release workflow, which builds and pushes the images.
#   2. Write the version to version/VERSION.txt and helm_chart/Chart.yaml,
#      commit, and push to main.
#   3. Package the Helm chart and attach it to a GitHub release.
#
# SKIP_GITHUB=true (automatic under `reactorcide run-local`) skips every push
# and the GitHub release, so the flow can run against a working tree.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$SRC_DIR"

SEMVER_TAGS_VERSION="v0.4.0"
GHCLI_VERSION="2.63.2"
REPO_SLUG="${REACTORCIDE_REPO:-catalystcommunity/catalystcal}"

if [ "${REACTORCIDE_WORKER_MODE:-remote}" = "local" ] && [ -z "${SKIP_GITHUB:-}" ]; then
    log "run-local detected: SKIP_GITHUB=true"
    SKIP_GITHUB=true
fi
SKIP_GITHUB="${SKIP_GITHUB:-false}"

if [ "$SKIP_GITHUB" != "true" ]; then
    [ -n "${GITHUB_PAT:-}" ] || die "GITHUB_PAT is not set"
    git config user.name "Catalyst Community (automation)"
    git config user.email "automation@catalystcommunity.dev"
    git remote set-url origin "https://x-access-token:${GITHUB_PAT}@github.com/${REPO_SLUG}.git"
    # runnerlib checks out a detached SHA. Land on main so the bump commit
    # has a branch to go on.
    git fetch --tags origin main
    git checkout -B main origin/main
fi

log "Install semver-tags ${SEMVER_TAGS_VERSION}"
curl -fsSL "https://github.com/catalystcommunity/semver-tags/releases/download/${SEMVER_TAGS_VERSION}/semver-tags.tar.gz" \
    -o /tmp/semver-tags.tar.gz
tar -xzf /tmp/semver-tags.tar.gz -C "$LOCAL_BIN"
chmod +x "$LOCAL_BIN/semver-tags"

log "Run semver-tags"
semver_args=(run --output_json)
[ "$SKIP_GITHUB" = "true" ] && semver_args+=(--dry_run)
# Print all output (push errors too) before the JSON tail is parsed.
semver-tags "${semver_args[@]}" > /tmp/semver-output.txt 2>&1 || true
cat /tmp/semver-output.txt
output="$(tail -1 /tmp/semver-output.txt)"
new_tag="$(echo "$output" | grep -o '"New_release_git_tag":"[^"]*"' | cut -d'"' -f4 || true)"
published="$(echo "$output" | grep -o '"New_release_published":"[^"]*"' | cut -d'"' -f4 || true)"

if [ "$published" != "true" ] && [ "$SKIP_GITHUB" != "true" ]; then
    echo "No new release needed."
    exit 0
fi
[ -n "$new_tag" ] || { echo "No new release tag."; exit 0; }
version="${new_tag#v}"
log "New release: ${new_tag}"

log "Write version ${version}"
echo "$version" > version/VERSION.txt
sed -i "s/^version: .*/version: ${version}/" helm_chart/Chart.yaml
sed -i "s/^appVersion: .*/appVersion: \"${version}\"/" helm_chart/Chart.yaml

if [ "$SKIP_GITHUB" != "true" ]; then
    git add version/VERSION.txt helm_chart/Chart.yaml
    if git diff --cached --quiet; then
        echo "No version changes to commit"
    else
        git commit -m "ci: bump version to ${version}"
        # Explicit refspec: `checkout -B` sets no upstream.
        git push origin HEAD:main
    fi
fi

install_helm
release_dir="/tmp/release"
rm -rf "$release_dir"
mkdir -p "$release_dir"
helm package helm_chart --destination "$release_dir"
ls -l "$release_dir"

if [ "$SKIP_GITHUB" = "true" ]; then
    log "SKIP_GITHUB=true: no GitHub release. Chart left in ${release_dir}"
    exit 0
fi

log "Create GitHub release ${new_tag}"
curl -fsSL "https://github.com/cli/cli/releases/download/v${GHCLI_VERSION}/gh_${GHCLI_VERSION}_linux_amd64.tar.gz" -o /tmp/gh.tar.gz
tar -xzf /tmp/gh.tar.gz -C /tmp
GH_TOKEN="$GITHUB_PAT" "/tmp/gh_${GHCLI_VERSION}_linux_amd64/bin/gh" release create "$new_tag" \
    --repo "$REPO_SLUG" \
    --title "$new_tag" \
    --generate-notes \
    "$release_dir"/*.tgz
log "Released ${new_tag}"
