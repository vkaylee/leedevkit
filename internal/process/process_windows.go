//go:build windows

package process

import (
	"os"
	"os/exec"
	"syscall"
)

func processGroupAttributes() *syscall.SysProcAttr { return nil }

func terminateProcess(process *os.Process) error { return process.Kill() }

func resultStatus(err *exec.ExitError) (int, bool) {
	return err.ExitCode(), true
}
