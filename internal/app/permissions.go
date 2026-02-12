package app

import (
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	PermissionsFullAccess            = "full-access"
	PermissionsDangerouslyFullAccess = "dangerously-full-access"
)

const sudoProbeCacheTTL = 5 * time.Second

var (
	sudoProbeMu       sync.Mutex
	sudoProbeAt       time.Time
	sudoProbeCachedOK bool

	probeNonInteractiveSudoFn = probeNonInteractiveSudo
)

// ParsePermissionsMode parses a user-provided permissions mode into a canonical value.
func ParsePermissionsMode(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")

	switch value {
	case "full-access", "full", "default", "current":
		return PermissionsFullAccess, true
	case "dangerously-full-access", "dangerously-full-acces", "dangerous", "dangerously", "root", "sudo":
		return PermissionsDangerouslyFullAccess, true
	default:
		return "", false
	}
}

// NormalizePermissionsMode returns a valid mode, defaulting to full-access.
func NormalizePermissionsMode(raw string) string {
	mode, ok := ParsePermissionsMode(raw)
	if !ok {
		return PermissionsFullAccess
	}
	return mode
}

// EffectivePermissionsMode returns the mode currently in effect and whether the process has elevated privileges.
//
// Elevated privileges means either running as root/admin, or having non-interactive sudo available.
func EffectivePermissionsMode(desired string) (effective string, isElevated bool) {
	normalized := NormalizePermissionsMode(desired)
	elevated := hasElevationCapability()
	if normalized == PermissionsDangerouslyFullAccess && !elevated {
		return PermissionsFullAccess, false
	}
	return normalized, elevated
}

func hasElevationCapability() bool {
	if IsProcessRoot() {
		return true
	}
	return nonInteractiveSudoAvailable()
}

func nonInteractiveSudoAvailable() bool {
	now := time.Now()

	sudoProbeMu.Lock()
	if !sudoProbeAt.IsZero() && now.Sub(sudoProbeAt) < sudoProbeCacheTTL {
		ok := sudoProbeCachedOK
		sudoProbeMu.Unlock()
		return ok
	}
	sudoProbeMu.Unlock()

	ok := probeNonInteractiveSudoFn()

	sudoProbeMu.Lock()
	sudoProbeCachedOK = ok
	sudoProbeAt = now
	sudoProbeMu.Unlock()
	return ok
}

// ResetElevationProbeCache clears the cached sudo/admin probe result.
func ResetElevationProbeCache() {
	sudoProbeMu.Lock()
	sudoProbeAt = time.Time{}
	sudoProbeCachedOK = false
	sudoProbeMu.Unlock()
}

func probeNonInteractiveSudo() bool {
	if _, err := exec.LookPath("sudo"); err != nil {
		return false
	}
	cmd := exec.Command("sudo", "-n", "true")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
