#!/usr/bin/env bash

set -euo pipefail

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

printf '\nInstalación desktop CLI Agent (Fedora)\n\n'

if [[ "${EUID}" -ne 0 ]]; then
  if [[ -z "${SUDO_PASSWORD:-}" ]]; then
    printf 'Se necesita acceso sudo para instalar dependencias de sistema.\n'
    read -r -s -p 'Contraseña sudo: ' SUDO_PASSWORD
    echo
  fi
fi

run_as_root() {
  if [[ -n "${SUDO_PASSWORD:-}" ]]; then
    printf '%s\n' "$SUDO_PASSWORD" | sudo -S "$@"
  else
    sudo "$@"
  fi
}

ensure_fedora() {
  if [[ ! -f /etc/os-release ]]; then
    echo "No se pudo detectar distro (/etc/os-release no encontrado)." >&2
    exit 1
  fi
  source /etc/os-release
  if [[ "${ID}" != "fedora" && "${ID_LIKE}" != *"fedora"* ]]; then
    echo "Este instalador es para Fedora (${PRETTY_NAME})." >&2
    exit 1
  fi
}

has_cmd() { command -v "$1" >/dev/null 2>&1; }

install_system_deps() {
  if has_cmd dnf; then
    :
  else
    echo "dnf no encontrado. Este instalador solo soporta Fedora." >&2
    exit 1
  fi

  echo "Instalando dependencias de sistema..."
  run_as_root dnf -y install git curl go nodejs npm tmux gcc make pkgconf-pkg-config gtk3-devel libappindicator-gtk3 libnotify-devel

  # WebKit puede variar por versión de Fedora; intentar ambos paquetes.
  if ! run_as_root dnf -y install webkit2gtk3-devel; then
    if ! run_as_root dnf -y install webkit2gtk4.1-devel; then
      echo "No pude instalar un paquete de WebKit para Wails (webkit2gtk3-devel / webkit2gtk4.1-devel)." >&2
      exit 1
    fi
  fi
}

install_wails() {
  local gobin
  gobin="$(go env GOPATH 2>/dev/null)/bin"
  if ! has_cmd go; then
    echo "Go no está disponible tras la instalación." >&2
    exit 1
  fi

  if ! has_cmd wails; then
    echo "Instalando CLI de Wails..."
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
  fi

  if ! has_cmd wails; then
    if [[ -x "$gobin/wails" ]]; then
      export PATH="${PATH}:${gobin}"
    fi
  fi

  if ! has_cmd wails; then
    echo "No se pudo ubicar el binario de Wails en PATH." >&2
    exit 1
  fi
}

build_desktop_app() {
  echo "Instalando dependencias del frontend..."
  (cd "$APP_DIR/frontend" && npm install)

  echo "Compilando UI (Vite) y binario Wails..."
  (cd "$APP_DIR" && wails build)
}

ensure_fedora
install_system_deps
install_wails
build_desktop_app

if [[ -n "${SUDO_PASSWORD:-}" ]]; then
  unset SUDO_PASSWORD
fi

echo -e "\nInstalación finalizada. Binario en ${APP_DIR}/build/zoodesk."
