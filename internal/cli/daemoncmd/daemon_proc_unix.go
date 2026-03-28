//go:build !windows

package daemoncmd

import (
	"os/exec"
	"syscall"
)

func configureDaemonChildProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
