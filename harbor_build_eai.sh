#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"

mkdir -p bin

echo "+ Building Harbor-compatible eai binary (CGO_DISABLED) -> bin/eai_harbor"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/eai_harbor ./cmd/eai/

echo "+ Verifying binary"
file bin/eai_harbor || true
ldd bin/eai_harbor 2>/dev/null || true

echo "ok: built bin/eai_harbor"

