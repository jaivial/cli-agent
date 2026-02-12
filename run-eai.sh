#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

source ./goenv.sh

# If you want to build a local binary instead:
#   go build -o ./eai ./cmd/eai/
#   ./eai
go run ./cmd/eai/

