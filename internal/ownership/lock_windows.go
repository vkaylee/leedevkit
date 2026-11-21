//go:build windows

package ownership

import "os"

func lockFile(file *os.File) error   { return nil }
func unlockFile(file *os.File) error { return nil }
