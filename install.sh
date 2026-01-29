#!/bin/bash
#
# EAI CLI Agent Installation Script
# Install the EAI CLI agent with a single command
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/clawdbot/clawd/main/cli-agent/install.sh | bash
#
# Options:
#   -b, --binary-url   URL to download binary from (default: auto-detect)
#   -d, --install-dir  Installation directory (default: /usr/local/bin)
#   -v, --version     Version to install (default: latest)
#   -h, --help        Show this help message
#
# Examples:
#   # Default installation
#   curl -fsSL https://raw.githubusercontent.com/clawdbot/clawd/main/cli-agent/install.sh | bash
#
#   # Install to custom directory
#   curl -fsSL https://raw.githubusercontent.com/clawdbot/clawd/main/cli-agent/install.sh | bash -s -- -d $HOME/.local/bin
#
#   # Install specific version
#   curl -fsSL https://raw.githubusercontent.com/clawdbot/clawd/main/cli-agent/install.sh | bash -s -- -v 1.0.0
#

set -e

# Configuration
REPO_OWNER="clawdbot"
REPO_NAME="clawd"
BINARY_NAME="eai"
INSTALL_DIR="/usr/local/bin"
BINARY_URL=""
VERSION="latest"
SCRIPT_VERSION="1.0.0"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

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
║                                                              ║
║                    CLI Agent Installer                        ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
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
                         (default: auto-detect from GitHub releases)
  -d, --install-dir DIR  Installation directory
                         (default: ${INSTALL_DIR})
  -v, --version VERSION  Version to install
                         (default: ${VERSION})
  -h, --help            Show this help message

Examples:
  # Default installation
  $ $0

  # Install to custom directory
  $ $0 -d \$HOME/.local/bin

  # Install specific version
  $ $0 -v 1.0.0

For more information, visit:
https://github.com/${REPO_OWNER}/${REPO_NAME}
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--binary-url)
            BINARY_URL="$2"
            shift 2
            ;;
        -d|--install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
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

# Detect operating system
detect_os() {
    case "$(uname -s)" in
        Linux*)
            OS="linux"
            ;;
        Darwin*)
            OS="darwin"
            ;;
        *)
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac
    log_info "Detected OS: ${OS}"
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)
            ARCH="x86_64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
    log_info "Detected architecture: ${ARCH}"
}

# Get latest release version from GitHub
get_latest_version() {
    if [[ "${VERSION}" == "latest" ]]; then
        log_info "Fetching latest version..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" 2>/dev/null | \
            grep '"tag_name"' | sed 's/.*"v\?\([^"]*\)".*/\1/' | head -1)
        
        if [[ -z "${VERSION}" ]]; then
            log_warn "Could not fetch latest version, using: ${VERSION}"
        else
            log_success "Latest version: ${VERSION}"
        fi
    fi
}

# Build binary URL if not provided
build_binary_url() {
    if [[ -z "${BINARY_URL}" ]]; then
        # Try to build from source if no pre-built binary
        log_info "No binary URL provided, checking for pre-built binaries..."
        
        # For now, we'll build from source
        log_info "Will build from source..."
        BINARY_URL="source"
    fi
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check curl or wget
    if command -v curl &> /dev/null; then
        DOWNLOADER="curl -fsSL"
    elif command -v wget &> /dev/null; then
        DOWNLOADER="wget -qO-"
    else
        log_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    # Check for Go if building from source
    if [[ "${BINARY_URL}" == "source" ]]; then
        if ! command -v go &> /dev/null; then
            log_error "Go is not installed. Please install Go from https://golang.org/dl/"
            exit 1
        fi
        
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        log_info "Go version: ${GO_VERSION}"
    fi
    
    # Check install directory is writable
    if [[ ! -d "${INSTALL_DIR}" ]]; then
        log_info "Creating installation directory: ${INSTALL_DIR}"
        mkdir -p "${INSTALL_DIR}"
    fi
    
    if [[ ! -w "${INSTALL_DIR}" ]]; then
        log_error "Cannot write to ${INSTALL_DIR}. Try running with sudo or use a different directory."
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Download and install binary
download_binary() {
    local binary_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    if [[ "${BINARY_URL}" == "source" ]]; then
        log_info "Building from source..."
        
        # Create temporary directory
        TEMP_DIR=$(mktemp -d)
        cd "${TEMP_DIR}"
        
        # Clone repository
        log_info "Cloning repository..."
        git clone --depth 1 --branch v${VERSION} "https://github.com/${REPO_OWNER}/${REPO_NAME}.git" 2>/dev/null || \
        git clone --depth 1 "https://github.com/${REPO_OWNER}/${REPO_NAME}.git" 2>/dev/null || \
        git clone --depth 1 "https://github.com/${REPO_OWNER}/${REPO_NAME}.git"
        
        cd "${REPO_NAME}/cli-agent"
        
        # Build binary
        log_info "Building binary..."
        CGO_ENABLED=0 go build -ldflags="-s -w" -o "${binary_path}" ./cmd/eai/
        
        # Cleanup
        cd /
        rm -rf "${TEMP_DIR}"
    else
        # Download pre-built binary
        log_info "Downloading binary from ${BINARY_URL}..."
        
        if ! $DOWNLOADER "${BINARY_URL}" -o "${binary_path}"; then
            log_error "Failed to download binary"
            exit 1
        fi
    fi
    
    # Make executable
    chmod +x "${binary_path}"
    
    log_success "Binary installed to ${binary_path}"
}

# Create default settings.json
create_settings() {
    local settings_path="${INSTALL_DIR}/settings.json"
    
    if [[ -f "${settings_path}" ]]; then
        log_info "Settings file already exists, skipping..."
        return 0
    fi
    
    log_info "Creating default settings file..."
    
    cat > "${settings_path}" << 'EOF'
# EAI CLI Agent Configuration
# Edit this file or run 'eai --configure' to set up your API key

# MiniMax API Configuration
minimax_api_key: ""
base_url: "https://api.minimax.io/anthropic/v1/messages"
model: "minimax-m2.1"
max_tokens: 2048

# Agent Configuration
max_parallel_agents: 50
default_mode: "plan"
safe_mode: true

# Installation
installed: false
EOF
    
    log_success "Settings file created at ${settings_path}"
}

# Setup shell completion (optional)
setup_completion() {
    log_info "Setting up shell completion..."
    
    local completion_dir=""
    local shell_name=""
    
    # Detect shell
    case "${SHELL##*/}" in
        bash)
            completion_dir="${HOME}/.bash_completion.d"
            shell_name="bash"
            ;;
        zsh)
            completion_dir="${HOME}/.zsh/completion"
            shell_name="zsh"
            ;;
        fish)
            completion_dir="${HOME}/.config/fish/completions"
            shell_name="fish"
            ;;
        *)
            log_warn "Unsupported shell: ${SHELL}. Skipping completion setup."
            return 0
            ;;
    esac
    
    # Create completion directory if needed
    if [[ ! -d "${completion_dir}" ]]; then
        mkdir -p "${completion_dir}"
    fi
    
    # Generate completion file
    local completion_file=""
    case "${shell_name}" in
        bash)
            completion_file="${completion_dir}/${BINARY_NAME}"
            ;;
        zsh)
            completion_file="${completion_dir}/_${BINARY_NAME}"
            ;;
        fish)
            completion_file="${completion_dir}/${BINARY_NAME}.fish"
            ;;
    esac
    
    # Use the built-in completion command
    "${INSTALL_DIR}/${BINARY_NAME}" completion "${shell_name}" > "${completion_file}" 2>/dev/null || \
    {
        log_warn "Could not generate completion file"
        return 0
    }
    
    log_success "Shell completion set up for ${shell_name}"
    
    # Print instructions
    echo ""
    log_info "To enable completion, add to your shell profile:"
    case "${shell_name}" in
        bash)
            echo "  echo 'source ${completion_file}' >> ~/.bashrc"
            ;;
        zsh)
            echo "  Add to ~/.zshrc: fpath=(\"${completion_dir}" \$fpath) && autoload -U compinit"
            ;;
        fish)
            echo "  Restart fish shell or run: source ${completion_file}"
            ;;
    esac
    echo ""
}

# Verify installation
verify_installation() {
    log_info "Verifying installation..."
    
    local binary_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    if [[ -x "${binary_path}" ]]; then
        log_success "Binary is executable"
    else
        log_error "Binary is not executable"
        exit 1
    fi
    
    # Test binary
    if "${binary_path}" --version &> /dev/null; then
        log_success "Binary works correctly"
        "${binary_path}" --version
    else
        log_warn "Binary test failed, but installation completed"
    fi
}

# Print post-installation message
print_summary() {
    cat << EOF

╔══════════════════════════════════════════════════════════════════════╗
║                                                              ║
║                    Installation Complete!                       ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝

Installed binary: ${INSTALL_DIR}/${BINARY_NAME}

To get started:
  1. Run 'eai' to start the CLI agent
  2. Type '/connect' to configure your MiniMax API key
  3. Enjoy!

To update later:
  curl -fsSL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/cli-agent/install.sh | bash

For more information:
  https://github.com/${REPO_OWNER}/${REPO_NAME}
EOF
}

# Main installation flow
main() {
    show_banner
    
    detect_os
    detect_arch
    get_latest_version
    build_binary_url
    check_prerequisites
    download_binary
    create_settings
    setup_completion
    verify_installation
    print_summary
}

# Run main function
main "$@"
