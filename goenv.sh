#!/usr/bin/env bash
set -euo pipefail

# Repo-local Go toolchain (no system install required).
#
# Usage:
#   source ./goenv.sh
#   go version

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
export GOROOT="$ROOT_DIR/.tools/go"
export PATH="$GOROOT/bin:$PATH"

