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

// detectState determines whether dir is a valid raw task directory. Raw means
// the test/Dockerfile is present (always required) and, for container tasks,
// the user/Dockerfile is also present. Git and vcluster tasks don't need a
// user image.
func detectState(dir string) taskState {
	testDockerfile := filepath.Join(dir, "test", "Dockerfile")
	if !fileExists(testDockerfile) {
		return stateUnknown
	}

	userDockerfile := filepath.Join(dir, "user", "Dockerfile")
	if fileExists(userDockerfile) {
		return stateRaw
	}

	// No user Dockerfile: only valid for git/vcluster tasks, which we detect
	// via task.yaml environment_type.
	cfg, err := loadTaskConfig(dir)
	if err == nil && (cfg.EnvironmentType == "git" || cfg.EnvironmentType == "vcluster") {
		return stateRaw
	}

	return stateUnknown
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
