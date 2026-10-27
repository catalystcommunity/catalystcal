#!/usr/bin/env bash
# helm lint and helm template for the chart, with the default values, the
# example values, and every optional part on.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$SRC_DIR"
install_helm
bash ./tools helm-lint
