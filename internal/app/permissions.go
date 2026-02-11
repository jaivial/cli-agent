package app

import (
	"os"
	"strings"
)

const (
	PermissionsFullAccess            = "full-access"
	PermissionsDangerouslyFullAccess = "dangerously-full-access"
)

var processEUID = os.Geteuid

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

func IsProcessRoot() bool {
	return processEUID() == 0
}

// EffectivePermissionsMode returns the mode currently in effect and whether the process is root.
// dangerously-full-access only becomes effective when running as root.
func EffectivePermissionsMode(desired string) (effective string, isRoot bool) {
	normalized := NormalizePermissionsMode(desired)
	root := IsProcessRoot()
	if normalized == PermissionsDangerouslyFullAccess && root {
		return PermissionsDangerouslyFullAccess, true
	}
	return PermissionsFullAccess, root
}
