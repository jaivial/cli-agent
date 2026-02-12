//go:build windows

package app

import "golang.org/x/sys/windows"

func IsProcessRoot() bool {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()

	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false
	}
	member, err := token.IsMember(adminSID)
	if err != nil {
		return false
	}
	return member
}
