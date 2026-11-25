//go:build windows

package process

import (
	"os"
	"os/exec"
	"syscall"
)

func processGroupAttributes() *syscall.SysProcAttr { return nil }

func signalProcessGroup(process *os.Process, _ bool) error { return process.Kill() }

func terminateProcess(process *os.Process) error { return process.Kill() }

func resultStatus(err *exec.ExitError) (int, bool) {
	return err.ExitCode(), true
}
