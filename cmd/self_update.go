package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const cliReleasesAPI = "https://api.github.com/repos/kagento/kagento-cli/releases"

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

var (
	selfUpdateCheck   bool
	selfUpdateJSON    bool
	selfUpdateVersion string
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Check for and install newer Kagento CLI releases",
	Args:  cobra.NoArgs,
	Run:   runSelfUpdate,
}

func init() {
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheck, "check", false, "Only check whether a newer version is available")
	selfUpdateCmd.Flags().BoolVar(&selfUpdateJSON, "json", false, "Output JSON")
	selfUpdateCmd.Flags().StringVar(&selfUpdateVersion, "version", "", "Install a specific version tag (for example v1.5.0)")
	rootCmd.AddCommand(selfUpdateCmd)
}

func runSelfUpdate(cmd *cobra.Command, args []string) {
	release, err := fetchRelease(selfUpdateVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking releases: %v\n", err)
		os.Exit(1)
	}

	current := normalizeVersion(version)
	target := normalizeVersion(release.TagName)
	upToDate := current != "" && current != "dev" && compareVersions(current, target) >= 0

	if selfUpdateCheck {
		if selfUpdateJSON {
			printJSON(map[string]any{
				"current_version": current,
				"target_version":  target,
				"up_to_date":      upToDate,
			})
			return
		}
		if upToDate {
			fmt.Printf("Kagento CLI is up to date (%s).\n", printableVersion(current))
		} else {
			fmt.Printf("New version available: %s (current: %s)\n", target, printableVersion(current))
		}
		return
	}

	if upToDate && selfUpdateVersion == "" {
		if selfUpdateJSON {
			printJSON(map[string]any{
				"current_version": current,
				"target_version":  target,
				"updated":         false,
				"up_to_date":      true,
			})
			return
		}
		fmt.Printf("Kagento CLI is already up to date (%s).\n", printableVersion(current))
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error locating executable: %v\n", err)
		os.Exit(1)
	}
	resolvedPath := execPath
	if realPath, err := filepath.EvalSymlinks(execPath); err == nil && realPath != "" {
		resolvedPath = realPath
	}
	if isHomebrewManaged(resolvedPath) {
		fmt.Fprintln(os.Stderr, "This installation is managed by Homebrew. Run 'brew upgrade kagento' instead.")
		os.Exit(1)
	}

	assetName := releaseAssetName()
	assetURL, err := releaseAssetURL(release, assetName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error selecting release asset: %v\n", err)
		os.Exit(1)
	}

	archivePath, err := downloadReleaseAsset(assetURL, assetName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading release: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = os.Remove(archivePath)
	}()

	binaryName := "kagento"
	if runtime.GOOS == "windows" {
		binaryName = "kagento.exe"
	}
	binaryBytes, err := extractBinaryFromArchive(archivePath, binaryName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting release binary: %v\n", err)
		os.Exit(1)
	}

	tmpPath := resolvedPath + ".tmp"
	if err := os.WriteFile(tmpPath, binaryBytes, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing updated binary: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Error setting executable permissions: %v\n", err)
		os.Exit(1)
	}
	if err := os.Rename(tmpPath, resolvedPath); err != nil {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Error replacing executable: %v\n", err)
		os.Exit(1)
	}

	if selfUpdateJSON {
		printJSON(map[string]any{
			"from_version": printableVersion(current),
			"path":         resolvedPath,
			"to_version":   target,
			"updated":      true,
		})
		return
	}
	fmt.Printf("Updated Kagento CLI from %s to %s\n", printableVersion(current), target)
}

func fetchRelease(tag string) (*githubRelease, error) {
	url := cliReleasesAPI + "/latest"
	if tag != "" {
		url = cliReleasesAPI + "/tags/" + normalizeVersion(tag)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}
	return &release, nil
}

func releaseAssetName() string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("kagento_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

func releaseAssetURL(release *githubRelease, assetName string) (string, error) {
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return asset.URL, nil
		}
	}
	return "", fmt.Errorf("release %s does not include %s", release.TagName, assetName)
}

func downloadReleaseAsset(url string, assetName string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	tmpFile, err := os.CreateTemp("", "kagento-self-update-*"+filepath.Ext(assetName))
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpFile.Close()
	}()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", err
	}
	return tmpFile.Name(), nil
}

func extractBinaryFromArchive(archivePath string, binaryName string) ([]byte, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractBinaryFromZip(archivePath, binaryName)
	}
	return extractBinaryFromTarGz(archivePath, binaryName)
}

func extractBinaryFromTarGz(archivePath string, binaryName string) ([]byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = gzReader.Close()
	}()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(header.Name) == binaryName {
			return io.ReadAll(tarReader)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", binaryName)
}

func extractBinaryFromZip(archivePath string, binaryName string) ([]byte, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	for _, file := range reader.File {
		if filepath.Base(file.Name) != binaryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = rc.Close()
		}()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("%s not found in archive", binaryName)
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

func printableVersion(v string) string {
	if strings.TrimSpace(v) == "" {
		return "unknown"
	}
	return v
}

func compareVersions(a, b string) int {
	parse := func(v string) []int {
		v = strings.TrimPrefix(normalizeVersion(v), "v")
		parts := strings.SplitN(v, "-", 2)
		fields := strings.Split(parts[0], ".")
		out := make([]int, 3)
		for i := 0; i < len(out) && i < len(fields); i++ {
			value, err := strconv.Atoi(fields[i])
			if err == nil {
				out[i] = value
			}
		}
		return out
	}

	av := parse(a)
	bv := parse(b)
	for i := range av {
		if av[i] < bv[i] {
			return -1
		}
		if av[i] > bv[i] {
			return 1
		}
	}
	return 0
}

func isHomebrewManaged(path string) bool {
	cleanPath := filepath.ToSlash(path)
	return strings.Contains(cleanPath, "/Cellar/kagento/") || strings.Contains(cleanPath, "/Homebrew/Cellar/kagento/")
}
