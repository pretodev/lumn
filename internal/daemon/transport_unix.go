//go:build !windows

package daemon

import (
	"context"
	"errors"
	"net"
	"os"
)

func windowsPipePath() string {
	return ""
}

func listenInternal(paths Paths) (net.Listener, error) {
	if err := os.Remove(paths.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return net.Listen("unix", paths.SocketPath)
}

func dialInternal(ctx context.Context, paths Paths) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, "unix", paths.SocketPath)
}

func cleanupInternal(paths Paths) error {
	if err := os.Remove(paths.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
