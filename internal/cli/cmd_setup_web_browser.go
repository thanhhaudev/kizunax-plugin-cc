package cli

import (
	"os/exec"
	"runtime"
)

// openInBrowser launches the user's default browser at url. Best-effort:
// if no opener is available or the call fails, the user can still copy
// the URL from stdout. Returns immediately; the launched browser process
// is detached.
func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}
