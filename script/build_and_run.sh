#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go run ./cmd/callbell-assets
dev_os="$(go env GOOS)"
dev_arch="$(go env GOARCH)"
dev_helper="resources/$dev_os-$dev_arch/bin/callbell-hook"
if [[ "$dev_os" == windows ]]; then dev_helper+=.exe; fi
mkdir -p "$(dirname "$dev_helper")"
CGO_ENABLED=0 go build -trimpath -o "$dev_helper" ./cmd/callbell-hook
exec go tool mygo dev "$@"
