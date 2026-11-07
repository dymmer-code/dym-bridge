package provision

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the address in the system browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux", "freebsd", "openbsd", "netbsd":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)
	}
	return cmd.Start()
}
