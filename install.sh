#!/usr/bin/env bash
#
# EAI CLI Agent Installer (install or update)
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/jaivial/cli-agent/main/install.sh | bash
#
# Notes:
# - Tries to install the latest GitHub release for your OS/arch (preferred).
# - Falls back to building from source if no matching release asset is found.
# - Attempts to install tmux (required for default multipane orchestration).
#
# Options:
#   -b, --binary-url URL     Direct URL to a prebuilt binary (or archive containing it)
#   -d, --install-dir DIR    Install directory (default: /usr/local/bin if writable, else ~/.local/bin)
#   -v, --version VERSION    Version/tag to install (default: latest; fallback: main)
#       --no-tmux-install    Do not attempt to auto-install tmux
#   -h, --help               Show help
#
set -euo pipefail

REPO_OWNER="jaivial"
REPO_NAME="cli-agent"
BINARY_NAME="eai"

INSTALL_DIR=""
BINARY_URL=""
VERSION="latest"
NO_TMUX_INSTALL="0"

OS=""
ARCH=""
RESOLVED_REF=""
RESOLVED_TAG=""

TEMP_DIRS=()

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }
die() { log_error "$1"; exit 1; }

cleanup() {
    local d
    for d in "${TEMP_DIRS[@]:-}"; do
        [[ -n "$d" ]] || continue
        rm -rf "$d" >/dev/null 2>&1 || true
    done
}

mktemp_dir() {
    local d
    d="$(mktemp -d 2>/dev/null || mktemp -d -t eai 2>/dev/null)" || die "mktemp failed"
    TEMP_DIRS+=("$d")
    printf '%s\n' "$d"
}

trap cleanup EXIT

show_banner() {
    cat << 'EOF'
╔══════════════════════════════════════════════════════════════════════╗
║   EAI CLI Agent Installer                                           ║
╚══════════════════════════════════════════════════════════════════════╝
EOF
    echo ""
}

show_help() {
    show_banner
    cat << EOF
Usage: $0 [OPTIONS]

Install (or update) the EAI CLI agent on your system.

Options:
  -b, --binary-url URL      Direct URL to a prebuilt binary (or archive)
  -d, --install-dir DIR     Install directory (default: auto)
  -v, --version VERSION     Version/tag to install (default: latest; fallback: main)
      --no-tmux-install     Do not attempt to auto-install tmux
  -h, --help                Show this help message

Examples:
  $ $0
  $ $0 -d \$HOME/.local/bin
  $ $0 -v v1.2.3

For more information:
https://github.com/${REPO_OWNER}/${REPO_NAME}
EOF
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--binary-url) BINARY_URL="${2:-}"; shift 2 ;;
        -d|--install-dir) INSTALL_DIR="${2:-}"; shift 2 ;;
        -v|--version) VERSION="${2:-}"; shift 2 ;;
        --no-tmux-install) NO_TMUX_INSTALL="1"; shift ;;
        -h|--help) show_help; exit 0 ;;
        *) die "Unknown option: $1" ;;
    esac
done

have_cmd() { command -v "$1" >/dev/null 2>&1; }

http_get() {
    local url="$1"
    if have_cmd curl; then
        curl -fsSL "$url"
        return 0
    fi
    if have_cmd wget; then
        wget -qO- "$url"
        return 0
    fi
    return 1
}

http_download() {
    local url="$1"
    local out="$2"
    if have_cmd curl; then
        curl -fsSL -o "$out" "$url"
        return 0
    fi
    if have_cmd wget; then
        wget -qO "$out" "$url"
        return 0
    fi
    return 1
}

detect_os() {
    local uname_s
    uname_s="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$uname_s" in
        linux*) OS="linux" ;;
        darwin*) OS="darwin" ;;
        *) die "Unsupported OS: $(uname -s)" ;;
    esac
    log_info "Detected OS: ${OS}"
}

detect_arch() {
    local uname_m
    uname_m="$(uname -m)"
    case "$uname_m" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) die "Unsupported architecture: ${uname_m}" ;;
    esac
    log_info "Detected architecture: ${ARCH}"
}

get_home_dir() {
    if [[ -n "${HOME:-}" ]]; then
        printf '%s\n' "${HOME}"
        return 0
    fi
    local h=""
    if have_cmd getent; then
        h="$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f6 || true)"
    fi
    if [[ -n "$h" ]]; then
        printf '%s\n' "$h"
        return 0
    fi
    printf '%s\n' "$(cd && pwd)"
}

set_default_install_dir() {
    if [[ -n "${INSTALL_DIR}" ]]; then
        return 0
    fi
    if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
        INSTALL_DIR="/usr/local/bin"
        return 0
    fi
    if [[ -d "/usr/local/bin" && -w "/usr/local/bin" ]]; then
        INSTALL_DIR="/usr/local/bin"
        return 0
    fi
    INSTALL_DIR="$(get_home_dir)/.local/bin"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    if ! have_cmd curl && ! have_cmd wget; then
        die "curl or wget is required"
    fi
    set_default_install_dir
    mkdir -p "${INSTALL_DIR}"
    if [[ ! -w "${INSTALL_DIR}" ]]; then
        die "Cannot write to ${INSTALL_DIR}. Re-run with sudo, or pass --install-dir to a writable directory (e.g. \$HOME/.local/bin)."
    fi
    if [[ "${NO_TMUX_INSTALL}" != "1" ]]; then
        install_tmux_if_missing
    fi
    log_success "Prerequisites passed"
}

install_tmux_if_missing() {
    if have_cmd tmux; then
        return 0
    fi

    log_info "tmux not found; attempting install..."
    if have_cmd apt-get; then
        if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
            apt-get update -y >/dev/null 2>&1 || true
            apt-get install -y tmux >/dev/null 2>&1 || true
        elif have_cmd sudo; then
            sudo apt-get update -y >/dev/null 2>&1 || true
            sudo apt-get install -y tmux >/dev/null 2>&1 || true
        fi
    elif have_cmd dnf; then
        if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
            dnf install -y tmux >/dev/null 2>&1 || true
        elif have_cmd sudo; then
            sudo dnf install -y tmux >/dev/null 2>&1 || true
        fi
    elif have_cmd yum; then
        if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
            yum install -y tmux >/dev/null 2>&1 || true
        elif have_cmd sudo; then
            sudo yum install -y tmux >/dev/null 2>&1 || true
        fi
    elif have_cmd pacman; then
        if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
            pacman -Sy --noconfirm tmux >/dev/null 2>&1 || true
        elif have_cmd sudo; then
            sudo pacman -Sy --noconfirm tmux >/dev/null 2>&1 || true
        fi
    elif have_cmd zypper; then
        if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
            zypper -n install tmux >/dev/null 2>&1 || true
        elif have_cmd sudo; then
            sudo zypper -n install tmux >/dev/null 2>&1 || true
        fi
    elif have_cmd brew; then
        brew install tmux >/dev/null 2>&1 || true
    fi

    if have_cmd tmux; then
        log_success "tmux installed"
    else
        log_warn "tmux is missing. eai will require tmux for default multipane orchestration (set EAI_NO_TMUX=1 to run without)."
    fi
}

fetch_release_json_latest() {
    http_get "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" 2>/dev/null || return 1
}

fetch_release_json_tag() {
    local tag="$1"
    http_get "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/tags/${tag}" 2>/dev/null || return 1
}

parse_tag_name() {
    local json="$1"
    echo "$json" | awk -F'"' '/"tag_name":/{print $4; exit}'
}

resolve_version_and_release() {
    RESOLVED_REF=""
    RESOLVED_TAG=""

    if [[ "${VERSION}" == "latest" ]]; then
        local json=""
        if json="$(fetch_release_json_latest)"; then
            local tag
            tag="$(parse_tag_name "$json")"
            if [[ -n "$tag" ]]; then
                RESOLVED_TAG="$tag"
                RESOLVED_REF="$tag"
                echo "$json"
                return 0
            fi
        fi
        RESOLVED_REF="main"
        return 1
    fi

    local v="${VERSION}"
    local json=""
    if json="$(fetch_release_json_tag "$v")"; then
        RESOLVED_TAG="$(parse_tag_name "$json")"
        RESOLVED_REF="${RESOLVED_TAG:-$v}"
        echo "$json"
        return 0
    fi
    if [[ "$v" != v* ]]; then
        if json="$(fetch_release_json_tag "v${v}")"; then
            RESOLVED_TAG="$(parse_tag_name "$json")"
            RESOLVED_REF="${RESOLVED_TAG:-v${v}}"
            echo "$json"
            return 0
        fi
    fi

    RESOLVED_REF="$v"
    return 1
}

select_release_asset_url() {
    local json="$1"
    local urls
    urls="$(echo "$json" | awk -F'"' '/browser_download_url/{print $4}')"

    local os_tokens=("${OS}")
    if [[ "${OS}" == "darwin" ]]; then
        os_tokens+=("macos" "osx" "mac")
    fi

    local arch_tokens=("${ARCH}")
    if [[ "${ARCH}" == "amd64" ]]; then
        arch_tokens+=("x86_64" "x64")
    fi
    if [[ "${ARCH}" == "arm64" ]]; then
        arch_tokens+=("aarch64")
    fi

    local best=""
    while IFS= read -r url; do
        [[ -n "$url" ]] || continue
        local lower
        lower="$(echo "$url" | tr '[:upper:]' '[:lower:]')"
        if [[ "$lower" == *checksums* || "$lower" == *.sha256* || "$lower" == *.sig* ]]; then
            continue
        fi
        [[ "$lower" == *"${BINARY_NAME}"* ]] || continue

        local os_ok="0"
        for tok in "${os_tokens[@]}"; do
            if [[ "$lower" == *"$tok"* ]]; then
                os_ok="1"
                break
            fi
        done
        [[ "$os_ok" == "1" ]] || continue

        local arch_ok="0"
        for tok in "${arch_tokens[@]}"; do
            if [[ "$lower" == *"$tok"* ]]; then
                arch_ok="1"
                break
            fi
        done
        [[ "$arch_ok" == "1" ]] || continue

        best="$url"
        break
    done <<< "$urls"

    echo "$best"
}

install_binary_from_path() {
    local src="$1"
    local dest="${INSTALL_DIR}/${BINARY_NAME}"

    if [[ ! -f "$src" ]]; then
        die "installer internal error: missing binary at $src"
    fi
    chmod +x "$src" 2>/dev/null || true

    local tmp="${dest}.tmp"
    cp "$src" "$tmp"
    chmod 0755 "$tmp" 2>/dev/null || true
    mv "$tmp" "$dest"

    log_success "Installed to ${dest}"
}

install_from_url() {
    local url="$1"

    local tmpdir
    tmpdir="$(mktemp_dir)"

    local lower
    lower="$(echo "$url" | tr '[:upper:]' '[:lower:]')"

    local archive="${tmpdir}/download"
    if [[ "$lower" == *.tar.gz ]]; then
        archive="${archive}.tar.gz"
    elif [[ "$lower" == *.tgz ]]; then
        archive="${archive}.tgz"
    elif [[ "$lower" == *.zip ]]; then
        archive="${archive}.zip"
    fi

    log_info "Downloading: ${url}"
    http_download "$url" "$archive" || die "Download failed"

    if [[ "$archive" == *.tar.gz || "$archive" == *.tgz ]]; then
        have_cmd tar || die "tar is required to extract ${archive}"
        tar -xzf "$archive" -C "$tmpdir"
    elif [[ "$archive" == *.zip ]]; then
        have_cmd unzip || die "unzip is required to extract ${archive}"
        unzip -q "$archive" -d "$tmpdir"
    else
        cp "$archive" "${tmpdir}/${BINARY_NAME}"
    fi

    local bin_path=""
    if have_cmd find; then
        bin_path="$(find "$tmpdir" -maxdepth 4 -type f -name "${BINARY_NAME}" 2>/dev/null | head -n1 || true)"
    fi
    if [[ -z "$bin_path" ]]; then
        die "Could not find ${BINARY_NAME} in downloaded artifact"
    fi

    install_binary_from_path "$bin_path"
}

build_from_source_tarball() {
    have_cmd go || die "Go is required to build from source (install Go 1.18+ or publish release binaries)."
    have_cmd tar || die "tar is required to build from source"

    local ref="$1"  # tag or branch name (e.g. v1.2.3, 1.2.3, main)

    local tmpdir
    tmpdir="$(mktemp_dir)"

    local src_tgz="${tmpdir}/src.tar.gz"
    local src_url=""
    local downloaded="0"
    local kinds=("tags" "heads")
    if [[ "$ref" == "main" || "$ref" == "master" ]]; then
        kinds=("heads")
    fi
    for kind in "${kinds[@]}"; do
        src_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/archive/refs/${kind}/${ref}.tar.gz"
        log_info "Downloading source: ${src_url}"
        if http_download "$src_url" "$src_tgz"; then
            downloaded="1"
            break
        fi
    done
    [[ "$downloaded" == "1" ]] || die "Failed to download source tarball for ref ${ref}"

    tar -xzf "$src_tgz" -C "$tmpdir"

    local src_dir=""
    if have_cmd find; then
        src_dir="$(find "$tmpdir" -maxdepth 1 -type d -name "${REPO_NAME}-*" 2>/dev/null | head -n1 || true)"
    fi
    if [[ -z "$src_dir" ]]; then
        die "Failed to locate extracted source directory"
    fi

    log_info "Building from source..."
    (cd "$src_dir" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "${tmpdir}/${BINARY_NAME}" ./cmd/eai/)
    install_binary_from_path "${tmpdir}/${BINARY_NAME}"
}

verify_installation() {
    local dest="${INSTALL_DIR}/${BINARY_NAME}"
    if [[ -x "$dest" ]]; then
        log_info "Verifying..."
        "$dest" --version 2>/dev/null || true
    fi
}

print_summary() {
    echo ""
    echo "Installation Complete!"
    echo "Binary: ${INSTALL_DIR}/${BINARY_NAME}"
    echo ""
    echo "To get started:"
    echo "  1. Run 'eai' to start the CLI agent"
    echo "  2. Type '/connect' to configure your API key"
    echo ""
    echo "To update later: rerun the same installer command."
    echo ""
    echo "Repo: https://github.com/${REPO_OWNER}/${REPO_NAME}"
    echo ""
}

main() {
    show_banner
    detect_os
    detect_arch
    check_prerequisites

    if [[ -n "${BINARY_URL}" ]]; then
        install_from_url "${BINARY_URL}"
        verify_installation
        print_summary
        return 0
    fi

    local release_json=""
    if release_json="$(resolve_version_and_release)"; then
        local tag
        tag="$(parse_tag_name "$release_json")"
        if [[ -n "$tag" ]]; then
            log_info "Resolved version: ${tag}"
        fi
        local asset_url
        asset_url="$(select_release_asset_url "$release_json")"
        if [[ -n "$asset_url" ]]; then
            log_info "Installing from release asset"
            install_from_url "$asset_url"
            verify_installation
            print_summary
            return 0
        fi
        log_warn "No matching release asset found for ${OS}/${ARCH}; falling back to source build."
    else
        log_warn "No GitHub release resolved; falling back to source build."
    fi

    local ref=""
    if [[ -n "${RESOLVED_TAG}" ]]; then
        ref="${RESOLVED_TAG}"
    elif [[ -n "${RESOLVED_REF}" ]]; then
        ref="${RESOLVED_REF}"
    else
        ref="main"
    fi
    build_from_source_tarball "$ref"
    verify_installation
    print_summary
}

main "$@"
