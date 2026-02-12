#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
harbor_prepull_images.sh

Pre-pull Docker images for Harbor Terminal-Bench tasks (reduces env-start timeouts).

Usage:
  ./harbor_prepull_images.sh [options] <task_name> [task_name...]
  ./harbor_prepull_images.sh --v19-failures

Options:
  --cache-dir PATH        Harbor tasks cache dir (default: ~/.cache/harbor/tasks)
  --download-terminal-bench
                          Runs: harbor datasets download terminal-bench@2.0
  --registry-url URL      Registry URL for dataset download (default: Harbor default)
  --dry-run               Print pulls without executing

Examples:
  ./harbor_prepull_images.sh qemu-startup qemu-alpine-ssh install-windows-3.11
  ./harbor_prepull_images.sh --v19-failures --download-terminal-bench --registry-url https://raw.githubusercontent.com/laude-institute/harbor/main/registry.json
EOF
}

die() {
  echo "error: $*" >&2
  exit 2
}

DRY_RUN=0
CACHE_DIR="${HOME}/.cache/harbor/tasks"
DOWNLOAD_TERMINAL_BENCH=0
REGISTRY_URL=""
TASKS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cache-dir)
      [[ -n "${2:-}" ]] || die "--cache-dir requires a value"
      CACHE_DIR="$2"
      shift 2
      ;;
    --download-terminal-bench)
      DOWNLOAD_TERMINAL_BENCH=1
      shift
      ;;
    --registry-url)
      [[ -n "${2:-}" ]] || die "--registry-url requires a value"
      REGISTRY_URL="$2"
      shift 2
      ;;
    --v19-failures)
      TASKS+=(
        caffe-cifar-10
        train-fasttext
        tune-mjcf
        qemu-startup
        qemu-alpine-ssh
        install-windows-3.11
        hf-model-inference
        pytorch-model-recovery
        mteb-retrieve
        mteb-leaderboard
        fix-ocaml-gc
      )
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      TASKS+=("$1")
      shift
      ;;
  esac
done

if [[ ${#TASKS[@]} -eq 0 ]]; then
  usage
  exit 2
fi

command -v docker >/dev/null 2>&1 || die "docker not found in PATH"
command -v harbor >/dev/null 2>&1 || die "harbor not found in PATH"

USE_SG_DOCKER=0
if docker info >/dev/null 2>&1; then
  USE_SG_DOCKER=0
elif command -v sg >/dev/null 2>&1 && getent group docker >/dev/null 2>&1; then
  if sg docker -c "docker info" >/dev/null 2>&1; then
    USE_SG_DOCKER=1
  else
    die "docker daemon not reachable (or permission denied)"
  fi
else
  die "docker daemon not reachable (or permission denied)"
fi

docker_cmd() {
  if [[ "${USE_SG_DOCKER}" -eq 1 ]]; then
    sg docker -c "$(printf '%q ' docker "$@")"
  else
    docker "$@"
  fi
}

if [[ "${DOWNLOAD_TERMINAL_BENCH}" -eq 1 ]]; then
  download_cmd=(harbor datasets download terminal-bench@2.0 --output-dir "${CACHE_DIR}")
  if [[ -n "${REGISTRY_URL}" ]]; then
    download_cmd+=(--registry-url "${REGISTRY_URL}")
  fi
  echo "+ ${download_cmd[*]}"
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    "${download_cmd[@]}"
  fi
fi

run() {
  echo "+ $*"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    return 0
  fi
  "$@"
}

find_task_dir() {
  local task_name="$1"
  ls -td "${CACHE_DIR}"/*/"${task_name}" 2>/dev/null | head -n 1 || true
}

extract_images_from_task() {
  local task_dir="$1"
  local toml_path="${task_dir}/task.toml"
  if [[ -r "${toml_path}" ]]; then
    awk -F'=' '/^[[:space:]]*docker_image[[:space:]]*=/ {print $2; exit}' "${toml_path}" \
      | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^"//' -e 's/"$//'
  fi

  local compose_path="${task_dir}/environment/docker-compose.yaml"
  if [[ -r "${compose_path}" ]]; then
    awk '/^[[:space:]]*image:[[:space:]]*/ {print $2}' "${compose_path}" \
      | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^"//' -e 's/"$//'
  fi
}

declare -A seen_images=()
images=()

for task in "${TASKS[@]}"; do
  task_dir="$(find_task_dir "${task}")"
  if [[ -z "${task_dir}" ]]; then
    echo "warn: task not found in cache: ${task}" >&2
    echo "      Hint: ./harbor_prepull_images.sh --download-terminal-bench --registry-url <registry> ${task}" >&2
    continue
  fi

  while IFS= read -r image; do
    [[ -n "${image}" ]] || continue
    [[ "${image}" != *'${'* ]] || continue
    if [[ -z "${seen_images[${image}]:-}" ]]; then
      images+=("${image}")
      seen_images["${image}"]=1
    fi
  done < <(extract_images_from_task "${task_dir}" || true)
done

if [[ ${#images[@]} -eq 0 ]]; then
  echo "No images found to pull."
  exit 0
fi

for image in "${images[@]}"; do
  if docker_cmd image inspect "${image}" >/dev/null 2>&1; then
    echo "ok: already present: ${image}"
    continue
  fi
  run docker_cmd pull "${image}"
done
