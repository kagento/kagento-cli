package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var taskPackCmd = &cobra.Command{
	Use:   "pack [path]",
	Short: "Create a portable {slug}.tar.gz archive from a task directory",
	Args:  cobra.MaximumNArgs(1),
	Run:   runTaskPack,
}

func init() {
	taskCmd.AddCommand(taskPackCmd)
}

func runTaskPack(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	cfg, err := loadTaskConfig(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Ensure built state: if raw, build first.
	state := detectState(dir)
	if state == stateRaw {
		fmt.Println("Raw state detected, building first...")
		if err := buildToTars(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else if state != stateBuilt {
		fmt.Fprintf(os.Stderr, "Error: directory is not in raw or built state\n")
		os.Exit(1)
	}

	// Verify built artifacts exist.
	for _, f := range []string{"task.yaml", "user.tar", "test.tar", filepath.Join("solution", "solve.sh")} {
		if !fileExists(filepath.Join(dir, f)) {
			fmt.Fprintf(os.Stderr, "Error: missing required file: %s\n", f)
			os.Exit(1)
		}
	}

	// Create {slug}.tar.gz.
	outPath := cfg.Slug + ".tar.gz"
	if err := createPortableArchive(dir, outPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	abs, _ := filepath.Abs(outPath)
	fmt.Printf("Portable archive: %s\n", abs)
}

// loadTaskConfig reads task.yaml without full validation (used when tars already exist).
func loadTaskConfig(dir string) (*TaskConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "task.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read task.yaml: %w", err)
	}

	var cfg TaskConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse task.yaml: %w", err)
	}

	if cfg.Slug == "" {
		return nil, fmt.Errorf("slug is required in task.yaml")
	}

	return &cfg, nil
}

// createPortableArchive packs task.yaml, solution/solve.sh, user.tar, test.tar into a .tar.gz.
func createPortableArchive(dir, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	files := []string{
		"task.yaml",
		filepath.Join("solution", "solve.sh"),
		"user.tar",
		"test.tar",
	}

	for _, name := range files {
		srcPath := filepath.Join(dir, name)
		if err := addFileToTar(tw, srcPath, name); err != nil {
			return fmt.Errorf("add %s: %w", name, err)
		}
	}

	return nil
}

// extractPortableArchive extracts a .tar.gz portable archive into a temp directory.
// Returns the path to the temp directory.
func extractPortableArchive(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	tmpDir, err := os.MkdirTemp("", "kagento-pack-*")
	if err != nil {
		return "", err
	}

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", err
		}

		target := filepath.Join(tmpDir, hdr.Name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			os.RemoveAll(tmpDir)
			return "", err
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(hdr.Mode))
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", err
		}

		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			os.RemoveAll(tmpDir)
			return "", err
		}
		out.Close()
	}

	return tmpDir, nil
}

func addFileToTar(tw *tar.Writer, srcPath, name string) error {
	fi, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name: name,
		Mode: int64(fi.Mode()),
		Size: fi.Size(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}
