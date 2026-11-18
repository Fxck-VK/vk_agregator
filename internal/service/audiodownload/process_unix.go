//go:build unix

package audiodownload

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func setProcessCancellation(cmd *exec.Cmd) {
	// Cancel FFmpeg and other subprocesses with yt-dlp.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
