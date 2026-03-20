package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

// taskState represents which form a task directory is in.
type taskState int

const (
	stateUnknown  taskState = iota
	stateRaw      // has user/Dockerfile + test/Dockerfile
	stateBuilt    // has user.tar + test.tar
	statePortable // a .tar.gz archive
)

func (s taskState) String() string {
	switch s {
	case stateRaw:
		return "raw"
	case stateBuilt:
		return "built"
	case statePortable:
		return "portable"
	default:
		return "unknown"
	}
}

// detectState determines whether dir is raw, built, or portable.
// For portable, pass the path to a .tar.gz file as dir.
func detectState(dir string) taskState {
	if strings.HasSuffix(dir, ".tar.gz") {
		if _, err := os.Stat(dir); err == nil {
			return statePortable
		}
	}

	userTar := filepath.Join(dir, "user.tar")
	testTar := filepath.Join(dir, "test.tar")
	if fileExists(userTar) && fileExists(testTar) {
		return stateBuilt
	}

	userDockerfile := filepath.Join(dir, "user", "Dockerfile")
	testDockerfile := filepath.Join(dir, "test", "Dockerfile")
	if fileExists(userDockerfile) && fileExists(testDockerfile) {
		return stateRaw
	}

	return stateUnknown
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
