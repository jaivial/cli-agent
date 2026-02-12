//go:build !windows

package app

import "os"

var processEUID = os.Geteuid

func IsProcessRoot() bool {
	return processEUID() == 0
}
