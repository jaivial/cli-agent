package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"cli-agent/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

type permissionsElevationResultMsg struct {
	text              string
	isError           bool
	sessionAuthorized bool
}

func (m *MainModel) permissionsModePostApplyCmd(mode string) tea.Cmd {
	mode = app.NormalizePermissionsMode(mode)
	switch mode {
	case app.PermissionsDangerouslyFullAccess:
		return m.authorizeDangerousPermissionsCmd()
	case app.PermissionsFullAccess:
		m.sessionElevationAuthorized = false
		return clearDangerousSessionCredentialsCmd()
	default:
		return nil
	}
}

func (m *MainModel) authorizeDangerousPermissionsCmd() tea.Cmd {
	app.ResetElevationProbeCache()
	effective, elevated := app.EffectivePermissionsMode(app.PermissionsDangerouslyFullAccess)
	if elevated && effective == app.PermissionsDangerouslyFullAccess {
		return func() tea.Msg {
			return permissionsElevationResultMsg{
				text:              "dangerously-full-access is active for this TUI session",
				sessionAuthorized: true,
			}
		}
	}

	cmd, err := buildElevationAuthorizeCommand()
	if err != nil {
		return func() tea.Msg {
			return permissionsElevationResultMsg{text: err.Error(), isError: true}
		}
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		app.ResetElevationProbeCache()
		if err != nil {
			return permissionsElevationResultMsg{
				text:    "authorization canceled or failed; dangerous mode is configured but not elevated",
				isError: true,
			}
		}

		if runtime.GOOS == "windows" {
			return permissionsElevationResultMsg{
				text:    "windows authorization accepted; reopen eai from the Administrator window to apply dangerous mode",
				isError: true,
			}
		}

		effective, elevated := app.EffectivePermissionsMode(app.PermissionsDangerouslyFullAccess)
		if elevated && effective == app.PermissionsDangerouslyFullAccess {
			return permissionsElevationResultMsg{
				text:              "dangerously-full-access authorized for this TUI session",
				sessionAuthorized: true,
			}
		}
		return permissionsElevationResultMsg{
			text:    "authorization completed, but elevated privileges are still unavailable",
			isError: true,
		}
	})
}

func clearDangerousSessionCredentialsCmd() tea.Cmd {
	if runtime.GOOS == "windows" {
		return nil
	}
	return func() tea.Msg {
		clearUnixElevationArtifacts()
		app.ResetElevationProbeCache()
		return nil
	}
}

func (m *MainModel) quitCmd() tea.Cmd {
	if !m.sessionElevationAuthorized {
		return tea.Quit
	}
	if runtime.GOOS == "windows" {
		return tea.Quit
	}
	return func() tea.Msg {
		clearUnixElevationArtifacts()
		app.ResetElevationProbeCache()
		return tea.Quit()
	}
}

func clearUnixElevationArtifacts() {
	if _, err := exec.LookPath("sudo"); err == nil {
		cmd := exec.Command("sudo", "-k")
		_ = cmd.Run()
	}
	if _, err := exec.LookPath("doas"); err == nil {
		cmd := exec.Command("doas", "-L")
		_ = cmd.Run()
	}
}

func buildElevationAuthorizeCommand() (*exec.Cmd, error) {
	script := ""
	switch runtime.GOOS {
	case "darwin":
		script = strings.TrimSpace(`
set -e
if ! command -v sudo >/dev/null 2>&1; then
  echo "sudo is not available on this system" >&2
  exit 127
fi
askpass="$(mktemp)"
cleanup() { rm -f "$askpass"; }
trap cleanup EXIT
cat >"$askpass" <<'ASKPASS'
#!/bin/sh
exec /usr/bin/osascript -e 'text returned of (display dialog "EAI needs your password to authorize this TUI session" default answer "" with hidden answer buttons {"Cancel", "Authorize"} default button "Authorize" with title "EAI Authorization")'
ASKPASS
chmod 700 "$askpass"
SUDO_ASKPASS="$askpass" sudo -A -v
`)
	case "linux":
		script = strings.TrimSpace(`
set -e
if command -v sudo >/dev/null 2>&1; then
  askpass="$(mktemp)"
  cleanup() { rm -f "$askpass"; }
  trap cleanup EXIT
  cat >"$askpass" <<'ASKPASS'
#!/bin/sh
if command -v zenity >/dev/null 2>&1; then
  exec zenity --password --title="EAI authorization"
fi
if command -v kdialog >/dev/null 2>&1; then
  exec kdialog --password "EAI authorization"
fi
if command -v ssh-askpass >/dev/null 2>&1; then
  exec ssh-askpass "EAI authorization"
fi
exit 127
ASKPASS
  chmod 700 "$askpass"
  if [ -n "${DISPLAY:-}" ] || [ -n "${WAYLAND_DISPLAY:-}" ]; then
    if SUDO_ASKPASS="$askpass" sudo -A -v; then
      exit 0
    fi
  fi
  sudo -v
  exit 0
fi

# Fedora/desktop fallback when sudo is not present.
if command -v pkexec >/dev/null 2>&1; then
  pkexec /bin/sh -c 'exit 0'
  exit $?
fi

# doas fallback (terminal password prompt).
if command -v doas >/dev/null 2>&1; then
  doas true
  exit $?
fi

echo "No supported elevation utility found (sudo/pkexec/doas)" >&2
exit 127
`)
	case "windows":
		ps := "powershell"
		if _, err := exec.LookPath(ps); err != nil {
			ps = "pwsh"
			if _, err := exec.LookPath(ps); err != nil {
				return nil, fmt.Errorf("powershell is not available to request Windows elevation")
			}
		}
		return exec.Command(ps, "-NoProfile", "-Command", "Start-Process -FilePath cmd.exe -Verb RunAs -ArgumentList '/c exit 0' -WindowStyle Hidden -Wait"), nil
	default:
		return nil, fmt.Errorf("automatic elevation is not supported on %s", runtime.GOOS)
	}

	shell := "bash"
	shellFlag := "-lc"
	if _, err := exec.LookPath(shell); err != nil {
		shell = "sh"
		shellFlag = "-c"
	}
	return exec.Command(shell, shellFlag, script), nil
}
