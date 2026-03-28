//go:build windows

package daemoncmd

import "os/exec"

func configureDaemonChildProcess(cmd *exec.Cmd) {}
