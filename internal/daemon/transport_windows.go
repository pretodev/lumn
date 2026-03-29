//go:build windows

package daemon

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

func windowsPipePath() string {
	return `\\.\pipe\lumnd`
}

func listenInternal(paths Paths) (net.Listener, error) {
	return winio.ListenPipe(windowsPipePath(), nil)
}

func dialInternal(ctx context.Context, paths Paths) (net.Conn, error) {
	return winio.DialPipeContext(ctx, windowsPipePath())
}

func cleanupInternal(paths Paths) error {
	return nil
}
