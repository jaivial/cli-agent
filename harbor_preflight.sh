#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
harbor_preflight.sh

Fast host preflight for Harbor Docker runs.

Usage:
  ./harbor_preflight.sh [--jobs-dir jobs] [--require-kvm]

Checks:
  - Docker CLI present and usable
  - SELinux mode (prints hints for Fedora bind mounts)
  - (Optional) /dev/kvm availability + permissions
EOF
}

die() {
  echo "error: $*" >&2
  exit 2
}

JOBS_DIR="jobs"
REQUIRE_KVM=0
USE_SG_DOCKER=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --jobs-dir)
      [[ -n "${2:-}" ]] || die "--jobs-dir requires a value"
      JOBS_DIR="$2"
      shift 2
      ;;
    --require-kvm)
      REQUIRE_KVM=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "Unknown arg: $1"
      ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "docker not found in PATH"

if docker info >/dev/null 2>&1; then
  USE_SG_DOCKER=0
elif command -v sg >/dev/null 2>&1 && getent group docker >/dev/null 2>&1; then
  if sg docker -c "docker info" >/dev/null 2>&1; then
    USE_SG_DOCKER=1
    echo "ok: docker access via \`sg docker -c ...\`"
  else
    echo "error: docker daemon not reachable (or permission denied)." >&2
    echo "       Fix: ensure Docker is running and your user can access /var/run/docker.sock." >&2
    exit 2
  fi
else
  echo "error: docker daemon not reachable (or permission denied)." >&2
  echo "       Fix: ensure Docker is running and your user can access /var/run/docker.sock." >&2
  exit 2
fi

if command -v getenforce >/dev/null 2>&1; then
  selinux_mode="$(getenforce 2>/dev/null || true)"
  if [[ -n "${selinux_mode}" && "${selinux_mode}" != "Disabled" ]]; then
    echo "ok: SELinux mode: ${selinux_mode}"
    echo "hint: if you see bind-mount write failures, run:"
    echo "      sudo chcon -Rt svirt_sandbox_file_t \"$(pwd)/${JOBS_DIR}\""
  fi
fi

if [[ "${REQUIRE_KVM}" -eq 1 ]]; then
  if [[ ! -e /dev/kvm ]]; then
    echo "error: /dev/kvm not found. QEMU/Windows tasks will be extremely slow or fail." >&2
    echo "       Fix: enable virtualization in BIOS and load KVM modules (kvm_intel/kvm_amd)." >&2
    exit 2
  fi

  if [[ ! -r /dev/kvm || ! -w /dev/kvm ]]; then
    echo "error: /dev/kvm exists but is not accessible by the current user." >&2
    echo "       Fix: add your user to the 'kvm' group (and re-login), or run with proper permissions." >&2
    ls -l /dev/kvm >&2 || true
    exit 2
  fi

  if [[ -r /proc/cpuinfo ]] && ! grep -Eq "(vmx|svm)" /proc/cpuinfo 2>/dev/null; then
    echo "warn: CPU virtualization flags (vmx/svm) not detected in /proc/cpuinfo." >&2
    echo "      Nested virtualization may be unavailable." >&2
  fi

  echo "ok: KVM available and accessible."
fi

echo "ok: preflight complete."
