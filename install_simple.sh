#!/bin/bash
#
# EAI CLI Agent Installation Script
# Install the EAI CLI agent with a single command
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/jaivial/cli-agent/master/install.sh | bash
#
# Options:
#   -b, --binary-url   URL to download binary from (default: auto-detect)
#   -d, --install-dir  Installation directory (default: /usr/local/bin)
#   -v, --version     Version to install (default: latest)
#   -h, --help        Show this help message
#
# Examples:
#   curl -fsSL https://raw.githubusercontent.com/jaivial/cli-agent/master/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/jaivial/cli-agent/master/install.sh | bash -s -- -d $HOME/.local/bin
#

set -e

REPO_OWNER="jaivial"
REPO_NAME="cli-agent"
BINARY_NAME="eai"
INSTALL_DIR="/usr/local/bin"
BINARY_URL=""
VERSION="latest"
SCRIPT_VERSION="1.0.0"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }

show_banner() {
    cat << 'EOF'
╔══════════════════════════════════════════════════════════════════════╗
║                                                              ║
║   ██╗██╗  ██╗███████╗ █████╗ ██████╗ ██╗   ██╗              ║
║   ██║██║ ██╔╝██╔════╝██╔══██╗██╔══██╗╚██╗ ██╔╝              ║
║   ██║█████╔╝ █████╗  ███████║██████╔╝ ╚████╔╝               ║
║   ██║██╔═██╗ ██╔══╝  ██╔══██║██╔══██╗  ╚██╔╝                ║
║   ██║██║  ██╗███████╗██║  ██║██║  ██║   ██║                  ║
║   ╚═╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝                  ║
║                    CLI Agent Installer                        ║
║                                                              ║
╚══════════════════════════════════════════════════════════════════════╝
EOF
    echo ""
}

show_help() {
    show_banner
    cat << EOF
Usage: $0 [OPTIONS]

Install the EAI CLI agent on your system.

Options:
  -b, --binary-url URL   URL to download binary from
  -d, --install-dir DIR  Installation directory (default: ${INSTALL_DIR})
  -v, --version VERSION  Version to install (default: ${VERSION})
  -h, --help            Show this help message

Examples:
  $ $0
  $ $0 -d \$HOME/.local/bin

For more information:
https://github.com/${REPO_OWNER}/${REPO_NAME}
EOF
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--binary-url) BINARY_URL="$2"; shift 2 ;;
        -d|--install-dir) INSTALL_DIR="$2"; shift 2 ;;
        -v|--version) VERSION="$2"; shift 2 ;;
        -h|--help) show_help; exit 0 ;;
        *) log_error "Unknown option: $1"; exit 1 ;;
    esac
done

detect_os() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$OS" in
        linux*) OS="linux" ;;
        darwin*) OS="darwin" ;;
        *) log_error "Unsupported OS: $OS"; exit 1 ;;
    esac
    log_info "Detected OS: ${OS}"
}

detect_arch() {
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64|amd64) ARCH="x86_64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) log_error "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    log_info "Detected architecture: ${ARCH}"
}

get_latest_version() {
    if [[ "${VERSION}" == "latest" ]]; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" 2>/dev/null | grep '"tag_name"' | sed 's/.*"v\?\([^"]*\)".*/\1/' | head -1)
        VERSION="${VERSION:-1.0.0}"
    fi
    log_success "Version: ${VERSION}"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    if ! command -v curl &> /dev/null && ! command -v wget &> /dev/null; then
        log_error "curl or wget required"
        exit 1
    fi
    if [[ "${BINARY_URL}" == "source" ]] && ! command -v go &> /dev/null; then
        log_error "Go required for building from source"
        exit 1
    fi
    if [[ ! -d "${INSTALL_DIR}" ]]; then
        mkdir -p "${INSTALL_DIR}"
    fi
    if [[ ! -w "${INSTALL_DIR}" ]]; then
        log_error "Cannot write to ${INSTALL_DIR}"
        exit 1
    fi
    install_tmux_if_missing
    log_success "Prerequisites passed"
}

install_tmux_if_missing() {
    if command -v tmux &> /dev/null; then
        return
    fi

    log_info "tmux not found; attempting silent install..."
    if command -v apt-get &> /dev/null; then
        if [[ "${EUID}" -eq 0 ]]; then
            if ! apt-get update -y > /dev/null || ! apt-get install -y tmux > /dev/null; then
                log_warn "tmux install via apt-get failed"
            fi
        elif command -v sudo &> /dev/null; then
            if ! sudo apt-get update -y > /dev/null || ! sudo apt-get install -y tmux > /dev/null; then
                log_warn "tmux install via sudo apt-get failed"
            fi
        else
            log_warn "tmux is required for pane orchestration; install it manually (apt-get or sudo unavailable)"
        fi
        return
    fi

    if command -v dnf &> /dev/null; then
        if [[ "${EUID}" -eq 0 ]]; then
            if ! dnf install -y tmux > /dev/null; then
                log_warn "tmux install via dnf failed"
            fi
        elif command -v sudo &> /dev/null; then
            if ! sudo dnf install -y tmux > /dev/null; then
                log_warn "tmux install via sudo dnf failed"
            fi
        else
            log_warn "tmux is required for pane orchestration; install it manually (dnf or sudo unavailable)"
        fi
        return
    fi

    if command -v brew &> /dev/null; then
        if ! brew install tmux > /dev/null; then
            log_warn "tmux install via brew failed"
        fi
        return
    fi

    log_warn "tmux missing and could not be auto-installed. Install tmux manually to enable pane orchestration."
}

download_binary() {
    local binary_path="${INSTALL_DIR}/${BINARY_NAME}"
    log_info "Building from source..."
    
    # Check Go version
    local go_version=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
    local major=$(echo "$go_version" | cut -d. -f1)
    local minor=$(echo "$go_version" | cut -d. -f2)
    
    if [[ "$major" -lt 1 ]] || [[ "$major" -eq 1 && "$minor" -lt 21 ]]; then
        log_info "Go version $go_version is too old, installing Go 1.21..."
        install_go_121
    fi
    
    local temp_dir=$(mktemp -d)
    local build_dir="${temp_dir}/build"
    
    # Check if we're already in the repository
    if [[ -f "cmd/eai/main.go" ]]; then
        log_info "Building from current directory..."
        build_dir="$(pwd)"
    else
        log_info "Cloning repository..."
        git clone --depth 1 "https://github.com/${REPO_OWNER}/${REPO_NAME}.git" "${temp_dir}/${REPO_NAME}"
        build_dir="${temp_dir}/${REPO_NAME}"
    fi
    
    cd "${build_dir}"
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "${binary_path}" ./cmd/eai/ 2>/dev/null || \
    go build -o "${binary_path}" ./cmd/eai/
    
    cd /
    rm -rf "${temp_dir}"
    
    chmod +x "${binary_path}"
    log_success "Installed to ${binary_path}"
}

install_go_121() {
    log_info "Installing Go 1.21..."
    local go_tar="go1.21.13.linux-amd64.tar.gz"
    local go_url="https://go.dev/dl/${go_tar}"
    
    cd /tmp
    curl -fsSL "$go_url" -o "$go_tar"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "$go_tar"
    rm "$go_tar"
    
    export PATH="/usr/local/go/bin:$PATH"
    export GOROOT="/usr/local/go"
    
    log_success "Go 1.21 installed"
}

create_settings() {
    local settings_path="${INSTALL_DIR}/settings.json"
    if [[ -f "${settings_path}" ]]; then
        return 0
    fi
    log_info "Creating settings file..."
    cat > "${settings_path}" << 'EOF'
# EAI CLI Agent Configuration
api_key: ""
base_url: "https://api.z.ai/api/paas/v4/chat/completions"
model: "glm-4.7"
max_tokens: 4096
max_parallel_agents: 50
default_mode: "orchestrate"
safe_mode: true
installed: false
EOF
    log_success "Settings created at ${settings_path}"
}

verify_installation() {
    log_info "Verifying..."
    local binary_path="${INSTALL_DIR}/${BINARY_NAME}"
    if [[ -x "${binary_path}" ]]; then
        log_success "Binary works"
        "${binary_path}" --version 2>/dev/null || true
    else
        log_warn "Binary may not work correctly"
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
    echo "For more info: https://github.com/${REPO_OWNER}/${REPO_NAME}"
    echo ""
}

main() {
    show_banner
    detect_os
    detect_arch
    get_latest_version
    check_prerequisites
    download_binary
    create_settings
    verify_installation
    print_summary
}

main "$@"
