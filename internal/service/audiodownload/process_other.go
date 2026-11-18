//go:build !unix

package audiodownload

import "os/exec"

func setProcessCancellation(_ *exec.Cmd) {
	// CommandContext cancels the downloader directly on non-Unix development systems.
}
