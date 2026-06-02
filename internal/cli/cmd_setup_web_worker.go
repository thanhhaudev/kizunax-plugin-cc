package cli

import (
	"net"
	"os"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

// runInternalSetupWebWorker is the detached child entry point spawned by
// setupWeb. It inherits the listener as fd 3 (ExtraFiles[0]) from the parent.
// It serves the form, writes config on save, and exits silently.
func runInternalSetupWebWorker(args []string) error {
	if len(args) < 1 {
		return xerrors.Internal("worker_args", "internal-setup-web-worker requires a token", nil)
	}
	token := args[0]

	f := os.NewFile(3, "setup-web-listener")
	if f == nil {
		return xerrors.Internal("worker_fd", "cannot reconstruct listener from fd 3", nil)
	}
	ln, err := net.FileListener(f)
	if err != nil {
		return xerrors.Internal("worker_listen", "cannot wrap inherited listener", err)
	}

	// Best-effort PID-file cleanup on every exit path.
	defer func() { _ = removeSetupWebPID() }()

	return serveSetupWeb(ln, token)
}

