package app

import "testing"

func TestParsePermissionsMode(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{in: "full-access", want: PermissionsFullAccess, ok: true},
		{in: "full", want: PermissionsFullAccess, ok: true},
		{in: "dangerously-full-access", want: PermissionsDangerouslyFullAccess, ok: true},
		{in: "dangerously-full-acces", want: PermissionsDangerouslyFullAccess, ok: true},
		{in: "sudo", want: PermissionsDangerouslyFullAccess, ok: true},
		{in: "unknown-mode", want: "", ok: false},
	}

	for _, tt := range tests {
		got, ok := ParsePermissionsMode(tt.in)
		if ok != tt.ok {
			t.Fatalf("ParsePermissionsMode(%q) ok=%v want %v", tt.in, ok, tt.ok)
		}
		if got != tt.want {
			t.Fatalf("ParsePermissionsMode(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestEffectivePermissionsMode(t *testing.T) {
	orig := processEUID
	defer func() { processEUID = orig }()

	processEUID = func() int { return 1000 }
	effective, isRoot := EffectivePermissionsMode(PermissionsDangerouslyFullAccess)
	if isRoot {
		t.Fatalf("expected non-root state")
	}
	if effective != PermissionsFullAccess {
		t.Fatalf("non-root dangerous mode should fall back to full-access, got %q", effective)
	}

	processEUID = func() int { return 0 }
	effective, isRoot = EffectivePermissionsMode(PermissionsDangerouslyFullAccess)
	if !isRoot {
		t.Fatalf("expected root state")
	}
	if effective != PermissionsDangerouslyFullAccess {
		t.Fatalf("root dangerous mode should be effective, got %q", effective)
	}
}

func TestDefaultConfigPermissions(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Permissions != PermissionsFullAccess {
		t.Fatalf("DefaultConfig permissions=%q want %q", cfg.Permissions, PermissionsFullAccess)
	}
}
