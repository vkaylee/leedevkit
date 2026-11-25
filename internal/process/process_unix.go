//go:build !windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func processGroupAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func signalProcessGroup(process *os.Process, force bool) error {
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(-process.Pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func terminateProcess(process *os.Process) error {
	if err := signalProcessGroup(process, false); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return signalProcessGroup(process, true)
}

func resultStatus(err *exec.ExitError) (int, bool) {
	status, ok := err.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, false
	}
	if status.Signaled() {
		return 128 + int(status.Signal()), true
	}
	return status.ExitStatus(), true
}
