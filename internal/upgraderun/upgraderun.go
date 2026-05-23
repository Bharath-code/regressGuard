// Package upgraderun implements the rg upgrade command.
// It checks GitHub releases for a newer version and optionally replaces
// the running binary in-place.
package upgraderun

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/ui"
)

const (
	repo           = "Bharath-code/regressguard"
	latestURL      = "https://api.github.com/repos/" + repo + "/releases/latest"
	checksumsFile  = "checksums.txt"
	httpTimeout    = 15 * time.Second
)

// Options configures an upgrade run.
type Options struct {
	CurrentVersion string
	CheckOnly      bool // --check: show available update without installing
	Stdout         io.Writer
	Stderr         io.Writer
}

// Result is the machine-readable outcome of rg upgrade.
type Result struct {
	Status         string `json:"status"`          // "updated", "up-to-date", "available"
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Message        string `json:"message"`
}

// githubRelease is the subset of the GitHub API response we need.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Run executes the upgrade pipeline.
func Run(opts Options) (Result, error) {
	opts = withDefaults(opts)

	// Fetch latest release info from GitHub.
	release, err := fetchLatestRelease()
	if err != nil {
		return Result{}, failures.Actionable{
			Title:       "rg upgrade failed: could not check for updates.",
			Cause:       err.Error(),
			Next:        "rg upgrade",
			MoreContext: "https://github.com/" + repo + "/releases",
		}
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(opts.CurrentVersion, "v")

	// Compare versions.
	if latestVersion == currentVersion || latestVersion == "" {
		result := Result{
			Status:         "up-to-date",
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Message:        fmt.Sprintf("rg %s is already the latest version.", currentVersion),
		}
		_, _ = fmt.Fprintln(opts.Stdout, paint(opts.Stdout, ui.ColorOK, ui.SymbolPass)+" "+result.Message)
		return result, nil
	}

	// Check-only mode.
	if opts.CheckOnly {
		result := Result{
			Status:         "available",
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Message:        fmt.Sprintf("Update available: %s -> %s", currentVersion, latestVersion),
		}
		_, _ = fmt.Fprintln(opts.Stdout, paint(opts.Stdout, ui.ColorInfo, ui.SymbolInfo)+" "+result.Message)
		_, _ = fmt.Fprintln(opts.Stdout)
		_, _ = fmt.Fprintln(opts.Stdout, "Run:")
		_, _ = fmt.Fprintln(opts.Stdout, "  "+paint(opts.Stdout, ui.ColorInfo, "rg upgrade"))
		return result, nil
	}

	// Determine the correct asset for this OS/arch.
	osName := runtime.GOOS
	archName := runtime.GOARCH
	archiveName := fmt.Sprintf("rg_%s_%s_%s.tar.gz", latestVersion, osName, archName)
	if osName == "windows" {
		archiveName = fmt.Sprintf("rg_%s_%s_%s.zip", latestVersion, osName, archName)
	}

	assetURL := ""
	checksumsURL := ""
	for _, asset := range release.Assets {
		if asset.Name == archiveName {
			assetURL = asset.BrowserDownloadURL
		}
		if asset.Name == checksumsFile {
			checksumsURL = asset.BrowserDownloadURL
		}
	}

	if assetURL == "" {
		return Result{}, failures.Actionable{
			Title:       "rg upgrade failed: no binary found for your platform.",
			Cause:       fmt.Sprintf("No asset matching %q in release %s.", archiveName, release.TagName),
			Next:        "https://github.com/" + repo + "/releases/tag/" + release.TagName,
			MoreContext: "rg version",
		}
	}

	// Show progress.
	_, _ = fmt.Fprintf(opts.Stderr, "%s Downloading rg %s (%s/%s)...\n", ui.SymbolRunning, latestVersion, osName, archName)

	// Download the archive to a temp file.
	tmpDir, err := os.MkdirTemp("", "rg-upgrade-*")
	if err != nil {
		return Result{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := tmpDir + "/" + archiveName
	if err := downloadFile(assetURL, archivePath); err != nil {
		return Result{}, failures.Actionable{
			Title:       "rg upgrade failed: download error.",
			Cause:       err.Error(),
			Next:        "rg upgrade",
			MoreContext: "https://github.com/" + repo + "/releases",
		}
	}

	// Verify checksum if available.
	if checksumsURL != "" {
		_, _ = fmt.Fprintf(opts.Stderr, "%s Verifying checksum...\n", ui.SymbolRunning)
		if err := verifyChecksum(checksumsURL, archivePath, archiveName); err != nil {
			return Result{}, failures.Actionable{
				Title:       "rg upgrade failed: checksum verification failed.",
				Cause:       err.Error(),
				Next:        "rg upgrade",
				MoreContext: "https://github.com/" + repo + "/releases",
			}
		}
	}

	// S5: verify GPG signature if available.
	sigURL := ""
	for _, asset := range release.Assets {
		if asset.Name == archiveName+".sig" || asset.Name == archiveName+".asc" {
			sigURL = asset.BrowserDownloadURL
			break
		}
	}
	if sigURL != "" && gpgAvailable() {
		_, _ = fmt.Fprintf(opts.Stderr, "%s Verifying GPG signature...\n", ui.SymbolRunning)
		if err := verifyGPGSignature(sigURL, archivePath, tmpDir); err != nil {
			_, _ = fmt.Fprintf(opts.Stderr, "%s GPG verification failed: %s (continuing with checksum only)\n", ui.SymbolWarning, err.Error())
		}
	}

	// Extract the binary.
	_, _ = fmt.Fprintf(opts.Stderr, "%s Extracting...\n", ui.SymbolRunning)
	binaryPath := tmpDir + "/rg"
	if osName == "windows" {
		binaryPath = tmpDir + "/rg.exe"
	}

	if err := extractArchive(archivePath, tmpDir, osName); err != nil {
		return Result{}, failures.Actionable{
			Title:       "rg upgrade failed: extraction error.",
			Cause:       err.Error(),
			Next:        "rg upgrade",
			MoreContext: "https://github.com/" + repo + "/releases",
		}
	}

	// Find the current binary path.
	currentBinary, err := os.Executable()
	if err != nil {
		return Result{}, fmt.Errorf("find current binary: %w", err)
	}
	// Resolve symlinks.
	currentBinary, err = resolveSymlink(currentBinary)
	if err != nil {
		return Result{}, fmt.Errorf("resolve binary path: %w", err)
	}

	// Replace the binary.
	_, _ = fmt.Fprintf(opts.Stderr, "%s Replacing %s...\n", ui.SymbolRunning, currentBinary)
	if err := replaceBinary(binaryPath, currentBinary); err != nil {
		return Result{}, failures.Actionable{
			Title:       "rg upgrade failed: could not replace binary.",
			Cause:       err.Error(),
			Next:        "sudo rg upgrade",
			MoreContext: "rg version",
		}
	}

	result := Result{
		Status:         "updated",
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        fmt.Sprintf("Updated: %s -> %s", currentVersion, latestVersion),
	}

	_, _ = fmt.Fprintln(opts.Stdout)
	_, _ = fmt.Fprintln(opts.Stdout, paint(opts.Stdout, ui.ColorOK, ui.SymbolPass)+" "+result.Message)
	_, _ = fmt.Fprintln(opts.Stdout)
	_, _ = fmt.Fprintln(opts.Stdout, "Verify:")
	_, _ = fmt.Fprintln(opts.Stdout, "  "+paint(opts.Stdout, ui.ColorInfo, "rg version"))

	return result, nil
}

// fetchLatestRelease queries the GitHub API for the latest release.
func fetchLatestRelease() (githubRelease, error) {
	client := &http.Client{Timeout: httpTimeout}
	req, err := http.NewRequest("GET", latestURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "regressguard-cli")

	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return githubRelease{}, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("parse release: %w", err)
	}
	return release, nil
}

// downloadFile downloads a URL to a local file path.
func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// verifyChecksum downloads the checksums file and verifies the archive.
func verifyChecksum(checksumsURL, archivePath, archiveName string) error {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(checksumsURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}

	// Find the expected hash for our archive.
	expectedHash := ""
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == archiveName {
			expectedHash = parts[0]
			break
		}
	}
	if expectedHash == "" {
		return fmt.Errorf("no checksum found for %s", archiveName)
	}

	// Compute actual hash.
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash[:16], actualHash[:16])
	}
	return nil
}

// extractArchive extracts the binary from a tar.gz or zip archive.
func extractArchive(archivePath, destDir, osName string) error {
	if osName == "windows" {
		// Use unzip for Windows archives.
		cmd := exec.Command("unzip", "-o", archivePath, "-d", destDir)
		return cmd.Run()
	}
	// Use tar for Unix archives.
	cmd := exec.Command("tar", "-xzf", archivePath, "-C", destDir)
	return cmd.Run()
}

// replaceBinary atomically replaces the target binary with the new one.
func replaceBinary(newPath, targetPath string) error {
	// Check if we can write to the target.
	if err := checkWritable(targetPath); err != nil {
		return err
	}

	// Read the new binary.
	newData, err := os.ReadFile(newPath)
	if err != nil {
		return fmt.Errorf("read new binary: %w", err)
	}

	// Write to a temp file next to the target, then rename (atomic on same fs).
	tmpPath := targetPath + ".new"
	if err := os.WriteFile(tmpPath, newData, 0o755); err != nil {
		return fmt.Errorf("write temp binary: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	return nil
}

// checkWritable verifies we can write to the target path.
func checkWritable(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("cannot write to %s (try: sudo rg upgrade)", path)
	}
	f.Close()
	return nil
}

// resolveSymlink resolves symlinks to get the actual binary path.
func resolveSymlink(path string) (string, error) {
	resolved, err := os.Readlink(path)
	if err != nil {
		// Not a symlink, return as-is.
		return path, nil
	}
	// If relative, resolve against the directory of the original path.
	if !strings.HasPrefix(resolved, "/") {
		dir := path[:strings.LastIndex(path, "/")+1]
		resolved = dir + resolved
	}
	return resolved, nil
}

func paint(w io.Writer, color ui.Color, text string) string {
	return ui.Paint(w, color, text)
}

// gpgAvailable checks if gpg is installed and accessible.
func gpgAvailable() bool {
	_, err := exec.LookPath("gpg")
	return err == nil
}

// verifyGPGSignature downloads the .sig/.asc file and verifies the archive.
// This provides supply-chain attack protection beyond SHA-256 checksums.
func verifyGPGSignature(sigURL, archivePath, tmpDir string) error {
	sigPath := tmpDir + "/archive.sig"
	if err := downloadFile(sigURL, sigPath); err != nil {
		return fmt.Errorf("download signature: %w", err)
	}

	// Verify the signature against the archive.
	cmd := exec.Command("gpg", "--verify", sigPath, archivePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg --verify failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func withDefaults(opts Options) Options {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return opts
}
