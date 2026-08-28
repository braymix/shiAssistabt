//go:build !windows

package supervisor

import (
	"os/exec"
	"syscall"
)

// setProcAttr puts the child in its own process group so we can signal the
// whole tree. prima.cpp's launchers may fork helper processes; killing only
// the direct child would orphan those (leaking ports and inherited fds).
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProc terminates the child and every process in its group. A negative pid
// addresses the process group (the child is its own group leader after
// Setpgid), so this reaps forked helpers too.
func killProc(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pgid := cmd.Process.Pid
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		// Fall back to signalling just the leader if the group is already gone.
		_ = cmd.Process.Kill()
	}
}
