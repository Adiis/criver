package chrome

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const InstallDir = "/opt/bro/bin"

// DownloadAndInstall downloads the chromedriver zip for the given version and
// platform, extracts the binary, and installs it to /opt/bro/bin/chromedriver.
func DownloadAndInstall(version, platform string) error {
	url := fmt.Sprintf(
		"https://storage.googleapis.com/chrome-for-testing-public/%s/%s/chromedriver-%s.zip",
		version, platform, platform,
	)

	tmpFile, err := os.CreateTemp("", "chromedriver-*.zip")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("saving zip: %w", err)
	}
	tmpFile.Close()

	binaryPath, err := extractChromedriver(tmpFile.Name(), platform)
	if err != nil {
		return err
	}
	defer os.Remove(binaryPath)

	if err := ensureInstallDir(); err != nil {
		return err
	}

	dest := filepath.Join(InstallDir, "chromedriver")
	if err := moveFile(binaryPath, dest); err != nil {
		return fmt.Errorf("installing binary: %w", err)
	}

	if err := os.Chmod(dest, 0755); err != nil {
		if out, serr := exec.Command("sudo", "chmod", "+x", dest).CombinedOutput(); serr != nil {
			return fmt.Errorf("chmod failed: %s: %w", string(out), err)
		}
	}

	return nil
}

func extractChromedriver(zipPath, platform string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	binaryName := "chromedriver"
	if platform == "win32" || platform == "win64" {
		binaryName = "chromedriver.exe"
	}

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name != binaryName {
			continue
		}
		if strings.Contains(f.Name, "..") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening zip entry: %w", err)
		}
		defer rc.Close()

		tmp, err := os.CreateTemp("", "chromedriver-bin-*")
		if err != nil {
			return "", fmt.Errorf("creating temp: %w", err)
		}

		if _, err := io.Copy(tmp, rc); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", fmt.Errorf("extracting: %w", err)
		}
		tmp.Close()
		return tmp.Name(), nil
	}

	return "", fmt.Errorf("chromedriver binary not found in zip")
}

func ensureInstallDir() error {
	if _, err := os.Stat(InstallDir); err == nil {
		return nil
	}
	if err := os.MkdirAll(InstallDir, 0755); err != nil {
		out, serr := exec.Command("sudo", "mkdir", "-p", InstallDir).CombinedOutput()
		if serr != nil {
			return fmt.Errorf("creating %s: %s: %w", InstallDir, string(out), serr)
		}
	}
	return nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, input, 0755); err != nil {
		tmpDst, terr := os.CreateTemp("", "chromedriver-install-*")
		if terr != nil {
			return err
		}
		tmpDst.Write(input)
		tmpDst.Close()

		out, serr := exec.Command("sudo", "cp", tmpDst.Name(), dst).CombinedOutput()
		os.Remove(tmpDst.Name())
		if serr != nil {
			return fmt.Errorf("sudo cp: %s: %w", string(out), serr)
		}
	}

	os.Remove(src)
	return nil
}

// IsInPath checks whether InstallDir is in the current PATH.
func IsInPath() bool {
	path := os.Getenv("PATH")
	for _, p := range filepath.SplitList(path) {
		if p == InstallDir {
			return true
		}
	}
	return false
}

// DetectShellRC returns the path to the user's shell rc file.
func DetectShellRC() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "/zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.HasSuffix(shell, "/bash"):
		bashrc := filepath.Join(home, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc
		}
		return filepath.Join(home, ".bash_profile")
	case strings.HasSuffix(shell, "/fish"):
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		for _, name := range []string{".zshrc", ".bashrc", ".bash_profile"} {
			rc := filepath.Join(home, name)
			if _, err := os.Stat(rc); err == nil {
				return rc
			}
		}
		return ""
	}
}

// AppendToPath appends the PATH export line to the given rc file.
func AppendToPath(rcFile string) error {
	exportLine := fmt.Sprintf("\n# Added by criver\nexport PATH=\"%s:$PATH\"\n", InstallDir)
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(exportLine)
	return err
}
