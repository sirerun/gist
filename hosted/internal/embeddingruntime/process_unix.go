//go:build darwin || linux

package embeddingruntime

import (
	"os/exec"
	"syscall"
)

func configureEmbeddingProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func signalEmbeddingProcess(cmd *exec.Cmd, kill bool) {
	signal := syscall.SIGTERM
	if kill {
		signal = syscall.SIGKILL
	}
	// Negative PID addresses only the private process group created at Start.
	_ = syscall.Kill(-cmd.Process.Pid, signal)
}
