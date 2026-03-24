package cmd

import (
	"os"
	"path/filepath"
)

// taskState represents which form a task directory is in.
type taskState int

const (
	stateUnknown taskState = iota
	stateRaw               // has user/Dockerfile + test/Dockerfile
)

func (s taskState) String() string {
	switch s {
	case stateRaw:
		return "raw"
	default:
		return "unknown"
	}
}

// detectState determines whether dir is a valid raw task directory.
func detectState(dir string) taskState {
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
