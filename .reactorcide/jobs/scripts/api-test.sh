#!/usr/bin/env bash
# gofmt, go vet, and go test for the coredb and api modules.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$SRC_DIR"
setup_go

log "gofmt -l"
unformatted="$(gofmt -l api coredb | grep -v "^api/internal/transport/" || true)"
if [ -n "$unformatted" ]; then
    echo "$unformatted"
    die "the files above need gofmt"
fi

for mod in coredb api; do
    log "go vet + go test: $mod"
    (cd "$mod" && go vet ./... && go test -count=1 ./...)
done

log "Build with the release flags"
(cd api && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.Version=ci-check" -o /tmp/catalystcal .)
reported="$(/tmp/catalystcal version)"
echo "catalystcal version: ${reported}"
case "$reported" in
    *ci-check*) ;;
    *) die "the binary does not report the -X main.Version value" ;;
esac
