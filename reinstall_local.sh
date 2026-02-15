#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./reinstall_local.sh [--out PATH] [--dest PATH] [--build-only] [--skip-tmux]

Builds eai from the local repo and (by default) installs it to ~/.local/bin/eai.

Options:
  --out PATH       Build output path (default: ./bin/eai)
  --dest PATH      Install destination file or directory (default: ~/.local/bin/eai)
  --build-only     Only build, do not install
  --skip-tmux      Skip tmux check/warning
  -h, --help       Show help
EOF
}

OUT=""
DEST=""
BUILD_ONLY="0"
SKIP_TMUX="0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) OUT="${2:-}"; shift 2 ;;
    --dest) DEST="${2:-}"; shift 2 ;;
    --build-only) BUILD_ONLY="1"; shift ;;
    --skip-tmux) SKIP_TMUX="1"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "error: unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$SCRIPT_DIR"
if [[ ! -f "${ROOT}/go.mod" && -f "${ROOT}/../go.mod" ]]; then
  ROOT="$(cd "${ROOT}/.." && pwd)"
fi
if [[ ! -f "${ROOT}/go.mod" ]]; then
  echo "error: could not find go.mod; run this from the repo (or place the script in the repo root)." >&2
  exit 1
fi

cd "$ROOT"

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required (Go 1.18+)." >&2
  exit 1
fi

if [[ "${SKIP_TMUX}" != "1" ]] && ! command -v tmux >/dev/null 2>&1; then
  echo "warn: tmux not found. eai's default multipane orchestration needs tmux." >&2
  echo "      Install it (e.g. Fedora: sudo dnf install -y tmux) or run with EAI_NO_TMUX=1." >&2
fi

if [[ -z "${OUT}" ]]; then
  OUT="${ROOT}/bin/eai"
fi

OUT_DIR="$(dirname "${OUT}")"
mkdir -p "${OUT_DIR}"

TMPDIR_BASE="${TMPDIR:-/tmp}"
TMPBIN="$(mktemp "${TMPDIR_BASE}/eai-build.XXXXXX" 2>/dev/null || mktemp -t eai-build)"
trap 'rm -f "${TMPBIN}" >/dev/null 2>&1 || true' EXIT

echo "[build] ${OUT}"
CGO_ENABLED="${CGO_ENABLED:-0}" go build -trimpath -ldflags="-s -w" -o "${TMPBIN}" ./cmd/eai/
chmod +x "${TMPBIN}" >/dev/null 2>&1 || true
mv "${TMPBIN}" "${OUT}"

echo "[ok] built ${OUT}"
"${OUT}" --version 2>/dev/null || true

if [[ "${BUILD_ONLY}" == "1" ]]; then
  exit 0
fi

HOME_DIR="${HOME:-$(cd ~ && pwd)}"
DEFAULT_DEST="${HOME_DIR}/.local/bin/eai"
if [[ -z "${DEST}" ]]; then
  DEST="${DEFAULT_DEST}"
fi

echo "[install] ${DEST}"
"${OUT}" install --dest "${DEST}"
echo "[ok] installed"

