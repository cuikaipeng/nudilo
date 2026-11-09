#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
package_os="$(go env GOOS)"
package_arch="$(go env GOARCH)"
if [[ "$package_os" == darwin ]]; then package_arch=universal; fi
exec python3 scripts/release.py package --platform "${PLATFORM:-$package_os/$package_arch}" "$@"
