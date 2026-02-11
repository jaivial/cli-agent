#!/usr/bin/env bash

set -euo pipefail

REPO_URL="https://github.com/jaivial/cli-agent.git"
BINARY_NAME="eai"
MIN_GO_MAJOR=1
MIN_GO_MINOR=18

INSTALL_MODE="user"
INSTALL_DIR=""
SOURCE_DIR=""
SKIP_DEPS=0
QUIET=0
UPDATE_PATH=1

CLONE_DIR=""
BUILD_DIR=""
PATH_UPDATED=0
PATH_RC_FILE=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    if [[ "${QUIET}" -eq 0 ]]; then
        echo -e "${BLUE}[INFO]${NC} $*"
    fi
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $*"
}

show_help() {
    cat <<'EOF'
Fedora installer for cli-agent (eai).

Usage:
  ./install_fedora.sh [options]

Options:
  --user               Install to ~/.local/bin (default)
  --system             Install to /usr/local/bin
  --install-dir DIR    Custom install directory
  --source DIR         Build from a local source directory
  --no-path-update     Do not modify shell profile PATH
  --skip-deps          Skip "dnf install" dependency step
  --quiet              Reduce output
  -h, --help           Show this help

Examples:
  ./install_fedora.sh
  ./install_fedora.sh --system
  ./install_fedora.sh --install-dir /opt/eai/bin
EOF
}

cleanup() {
    if [[ -n "${BUILD_DIR}" && -d "${BUILD_DIR}" ]]; then
        rm -rf "${BUILD_DIR}"
    fi
    if [[ -n "${CLONE_DIR}" && -d "${CLONE_DIR}" ]]; then
        rm -rf "${CLONE_DIR}"
    fi
}
trap cleanup EXIT

require_cmd() {
    local cmd="$1"
    if ! command -v "${cmd}" >/dev/null 2>&1; then
        log_error "Missing required command: ${cmd}"
        exit 1
    fi
}

run_as_root() {
    if [[ "${EUID}" -eq 0 ]]; then
        "$@"
        return
    fi
    if command -v sudo >/dev/null 2>&1; then
        sudo "$@"
        return
    fi
    log_error "This step needs root privileges; install sudo or run as root."
    exit 1
}

target_user() {
    if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
        echo "${SUDO_USER}"
    else
        id -un
    fi
}

target_home() {
    local user
    user="$(target_user)"
    local home_dir
    home_dir="$(getent passwd "${user}" | cut -d: -f6 || true)"
    if [[ -z "${home_dir}" ]]; then
        home_dir="${HOME}"
    fi
    echo "${home_dir}"
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --user)
                INSTALL_MODE="user"
                shift
                ;;
            --system)
                INSTALL_MODE="system"
                shift
                ;;
            --install-dir)
                INSTALL_DIR="${2:-}"
                if [[ -z "${INSTALL_DIR}" ]]; then
                    log_error "--install-dir requires a value"
                    exit 1
                fi
                shift 2
                ;;
            --source)
                SOURCE_DIR="${2:-}"
                if [[ -z "${SOURCE_DIR}" ]]; then
                    log_error "--source requires a value"
                    exit 1
                fi
                shift 2
                ;;
            --no-path-update)
                UPDATE_PATH=0
                shift
                ;;
            --skip-deps)
                SKIP_DEPS=1
                shift
                ;;
            --quiet)
                QUIET=1
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

ensure_fedora() {
    if [[ ! -r /etc/os-release ]]; then
        log_error "Cannot verify distro: /etc/os-release not found."
        exit 1
    fi

    # shellcheck disable=SC1091
    source /etc/os-release
    if [[ "${ID:-}" != "fedora" && "${ID_LIKE:-}" != *fedora* ]]; then
        log_error "This installer is for Fedora only. Detected: ${PRETTY_NAME:-unknown}"
        exit 1
    fi
    log_success "Detected Fedora (${PRETTY_NAME:-fedora})"
}

default_install_dir() {
    if [[ -n "${INSTALL_DIR}" ]]; then
        return
    fi
    if [[ "${INSTALL_MODE}" == "system" ]]; then
        INSTALL_DIR="/usr/local/bin"
        return
    fi
    INSTALL_DIR="$(target_home)/.local/bin"
}

install_dependencies() {
    if [[ "${SKIP_DEPS}" -eq 1 ]]; then
        log_warn "Skipping dependency installation (--skip-deps)."
        return
    fi

    require_cmd dnf
    log_info "Installing dependencies with dnf (git, golang, curl)..."
    run_as_root dnf -y install git golang curl
}

check_go_version() {
    require_cmd go

    local goversion raw major minor
    goversion="$(go env GOVERSION 2>/dev/null || true)"
    if [[ -z "${goversion}" ]]; then
        goversion="$(go version | awk '{print $3}')"
    fi

    raw="${goversion#go}"
    IFS='.' read -r major minor _ <<<"${raw}"

    if [[ -z "${major}" || -z "${minor}" ]]; then
        log_error "Could not parse Go version (${goversion})."
        exit 1
    fi

    if (( major < MIN_GO_MAJOR )) || (( major == MIN_GO_MAJOR && minor < MIN_GO_MINOR )); then
        log_error "Go ${MIN_GO_MAJOR}.${MIN_GO_MINOR}+ required, found ${raw}. Upgrade with: sudo dnf upgrade golang"
        exit 1
    fi
    log_success "Go version ${raw} is supported"
}

resolve_source_dir() {
    if [[ -n "${SOURCE_DIR}" ]]; then
        SOURCE_DIR="$(cd "${SOURCE_DIR}" && pwd)"
    elif [[ -f "./go.mod" && -f "./cmd/eai/main.go" ]]; then
        SOURCE_DIR="$(pwd)"
    else
        require_cmd git
        CLONE_DIR="$(mktemp -d)"
        log_info "Cloning source from ${REPO_URL}..."
        git clone --depth 1 "${REPO_URL}" "${CLONE_DIR}/cli-agent"
        SOURCE_DIR="${CLONE_DIR}/cli-agent"
    fi

    if [[ ! -f "${SOURCE_DIR}/go.mod" || ! -f "${SOURCE_DIR}/cmd/eai/main.go" ]]; then
        log_error "Invalid source directory: ${SOURCE_DIR}"
        exit 1
    fi
    log_success "Using source directory: ${SOURCE_DIR}"
}

build_binary() {
    BUILD_DIR="$(mktemp -d)"
    log_info "Building ${BINARY_NAME}..."

    (
        cd "${SOURCE_DIR}"
        CGO_ENABLED=0 go build -o "${BUILD_DIR}/${BINARY_NAME}" ./cmd/eai
    )

    if [[ ! -x "${BUILD_DIR}/${BINARY_NAME}" ]]; then
        log_error "Build failed; binary not found."
        exit 1
    fi
    log_success "Build complete"
}

install_binary() {
    local dest="${INSTALL_DIR}/${BINARY_NAME}"

    if mkdir -p "${INSTALL_DIR}" 2>/dev/null && install -m 0755 "${BUILD_DIR}/${BINARY_NAME}" "${dest}" 2>/dev/null; then
        :
    else
        run_as_root mkdir -p "${INSTALL_DIR}"
        run_as_root install -m 0755 "${BUILD_DIR}/${BINARY_NAME}" "${dest}"
    fi

    if [[ -n "${SUDO_USER:-}" && "${INSTALL_MODE}" == "user" ]]; then
        run_as_root chown "${SUDO_USER}:${SUDO_USER}" "${dest}"
    fi

    log_success "Installed binary: ${dest}"
}

target_shell() {
    local user shell_path
    user="$(target_user)"
    shell_path="$(getent passwd "${user}" | cut -d: -f7 || true)"
    if [[ -z "${shell_path}" ]]; then
        shell_path="/bin/bash"
    fi
    echo "${shell_path}"
}

ensure_path_update() {
    if [[ "${UPDATE_PATH}" -eq 0 ]]; then
        return
    fi

    # System installs typically land in already-present PATH.
    if [[ "${INSTALL_MODE}" == "system" ]]; then
        return
    fi

    # Never auto-add world-writable temp dirs to PATH.
    if [[ "${INSTALL_DIR}" == "/tmp" || "${INSTALL_DIR}" == /tmp/* ]]; then
        log_warn "Skipping PATH update for insecure temporary directory: ${INSTALL_DIR}"
        return
    fi

    local home_dir shell_path shell_name rc_file export_line rc_dir
    home_dir="$(target_home)"
    shell_path="$(target_shell)"
    shell_name="$(basename "${shell_path}")"

    case "${shell_name}" in
        zsh)
            rc_file="${home_dir}/.zshrc"
            export_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
            ;;
        fish)
            rc_file="${home_dir}/.config/fish/config.fish"
            export_line="set -gx PATH ${INSTALL_DIR} \$PATH"
            ;;
        *)
            rc_file="${home_dir}/.bashrc"
            export_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
            ;;
    esac

    rc_dir="$(dirname "${rc_file}")"

    if [[ -f "${rc_file}" ]] && grep -Fq "${INSTALL_DIR}" "${rc_file}"; then
        PATH_RC_FILE="${rc_file}"
        log_info "PATH already configured in ${rc_file}"
        return
    fi

    if [[ -w "${rc_dir}" || ! -e "${rc_dir}" ]]; then
        mkdir -p "${rc_dir}"
        {
            echo ""
            echo "# Added by cli-agent Fedora installer"
            echo "${export_line}"
        } >> "${rc_file}"
    else
        run_as_root mkdir -p "${rc_dir}"
        {
            echo ""
            echo "# Added by cli-agent Fedora installer"
            echo "${export_line}"
        } | run_as_root tee -a "${rc_file}" >/dev/null
    fi

    if [[ -n "${SUDO_USER:-}" ]]; then
        run_as_root chown "${SUDO_USER}:${SUDO_USER}" "${rc_file}" 2>/dev/null || true
    fi

    PATH_UPDATED=1
    PATH_RC_FILE="${rc_file}"
    log_success "Added ${INSTALL_DIR} to PATH in ${rc_file}"
}

create_default_config() {
    local home_dir cfg_dir cfg_file
    home_dir="$(target_home)"
    cfg_dir="${home_dir}/.config/cli-agent"
    cfg_file="${cfg_dir}/config.yml"

    if [[ -f "${cfg_file}" ]]; then
        log_info "Config already exists: ${cfg_file}"
        return
    fi

    mkdir -p "${cfg_dir}"
    cat > "${cfg_file}" <<'EOF'
api_key: ""
base_url: "https://api.z.ai/api/paas/v4/chat/completions"
model: "glm-4.7"
max_tokens: 4096
max_parallel_agents: 50
mode: "plan"
safe_mode: true
EOF

    if [[ -n "${SUDO_USER:-}" ]]; then
        run_as_root chown -R "${SUDO_USER}:${SUDO_USER}" "${cfg_dir}"
    fi
    log_success "Created default config: ${cfg_file}"
}

verify_install() {
    local bin_path="${INSTALL_DIR}/${BINARY_NAME}"
    if [[ ! -x "${bin_path}" ]]; then
        log_error "Installed binary is not executable: ${bin_path}"
        exit 1
    fi
    log_info "Version check:"
    "${bin_path}" --version || true
}

print_next_steps() {
    local home_dir shell_rc
    home_dir="$(target_home)"
    shell_rc="${home_dir}/.bashrc"

    echo
    echo "Install complete."
    echo "Binary: ${INSTALL_DIR}/${BINARY_NAME}"
    echo "Config: ${home_dir}/.config/cli-agent/config.yml"

    if [[ "${PATH_UPDATED}" -eq 1 && -n "${PATH_RC_FILE}" ]]; then
        echo
        echo "PATH was updated in ${PATH_RC_FILE}."
        echo "Run to apply now:"
        echo "  source ${PATH_RC_FILE}"
    else
        case ":${PATH}:" in
            *":${INSTALL_DIR}:"*) ;;
            *)
                echo
                echo "Your PATH does not include ${INSTALL_DIR}."
                echo "Add this line to ${shell_rc}:"
                echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
                ;;
        esac
    fi

    echo
    echo "Run:"
    echo "  ${INSTALL_DIR}/${BINARY_NAME}"
    echo "Then set your API key with /connect in the TUI or:"
    echo "  export EAI_API_KEY=\"your-key\""
}

main() {
    parse_args "$@"
    ensure_fedora
    default_install_dir
    install_dependencies
    check_go_version
    resolve_source_dir
    build_binary
    install_binary
    ensure_path_update
    create_default_config
    verify_install
    print_next_steps
}

main "$@"
