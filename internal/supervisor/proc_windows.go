//go:build windows

package supervisor

import "os/exec"

// setProcAttr is a no-op on Windows, which has no POSIX process groups. The
// child is launched normally.
func setProcAttr(cmd *exec.Cmd) {}

// killProc terminates the child process. Windows does not give us a cheap way
// to reap the whole tree here; the direct child is killed.
func killProc(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
