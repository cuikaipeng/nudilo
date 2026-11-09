#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
test_os="$(go env GOOS)"
test_arch="$(go env GOARCH)"
if [[ "$test_os" != darwin ]]; then
  echo 'Native automation smoke currently runs on macOS; use make test-ui on other systems.' >&2
  exit 1
fi
mkdir -p "resources/$test_os-$test_arch/bin"
CGO_ENABLED=0 go build -trimpath -o "resources/$test_os-$test_arch/bin/callbell-hook" ./cmd/callbell-hook
GOFLAGS='-tags=e2e' go tool mygo build -debug -skip-dmg -o build/e2e
python3 scripts/smoke.py "build/e2e/$test_os-$test_arch/Nudilo.app/Contents/MacOS/Nudilo"
