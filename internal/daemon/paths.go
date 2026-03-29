package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Paths struct {
	StateDir   string
	DBPath     string
	ConfigPath string
	SocketPath string
	PIDPath    string
	LogPath    string
}

func DefaultPaths() (Paths, error) {
	stateDir := os.Getenv("LUMN_STATE_DIR")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
		stateDir = filepath.Join(home, ".lumn")
	}

	paths := Paths{
		StateDir:   stateDir,
		DBPath:     filepath.Join(stateDir, "lumnd.db"),
		ConfigPath: filepath.Join(stateDir, "lumnd.conf"),
		PIDPath:    filepath.Join(stateDir, "lumnd.pid"),
		LogPath:    filepath.Join(stateDir, "lumnd.log"),
	}
	if runtime.GOOS != "windows" {
		paths.SocketPath = filepath.Join(stateDir, "lumnd.sock")
	}
	return paths, nil
}

func (p Paths) EnsureStateDir() error {
	return os.MkdirAll(p.StateDir, 0o755)
}

func (p Paths) TransportDescription() string {
	if runtime.GOOS == "windows" {
		return windowsPipePath()
	}
	return p.SocketPath
}

func (p Paths) InternalURL() string {
	return "http://lumn"
}

func (p Paths) String() string {
	return fmt.Sprintf("state=%s transport=%s", p.StateDir, p.TransportDescription())
}
