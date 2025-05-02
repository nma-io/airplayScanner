package updater

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type UpdateConfig struct {
	VersionURL     string
	BinaryURL      string
	CurrentVersion string
	BinaryName     string
	Timeout        time.Duration
	OSMap          map[string]string
	ArchMap        map[string]string
}

func NewUpdateConfig(versionURL, binaryURL, currentVersion string) *UpdateConfig {
	return &UpdateConfig{
		VersionURL:     versionURL,
		BinaryURL:      binaryURL,
		CurrentVersion: currentVersion,
		BinaryName:     filepath.Base(os.Args[0]),
		Timeout:        20 * time.Second,
		OSMap: map[string]string{
			"darwin":  "osx",
			"linux":   "elf",
			"windows": "exe",
		},
		ArchMap: map[string]string{
			"amd64": "x86",
			"arm64": "arm",
			"386":   "ish", // 32bit linux for ipad Ish Linux Emulator. :)
		},
	}
}

func (c *UpdateConfig) GetPlatformBinaryURL() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	osName := c.OSMap[os]
	archName := c.ArchMap[arch]

	if osName == "" || archName == "" {
		return c.BinaryURL
	}

	baseURL := strings.TrimSuffix(c.BinaryURL, "/")
	return fmt.Sprintf("%s/airplayScanner.%s.%s", baseURL, osName, archName)
}

func Update(currentVersion string) error {
	config := NewUpdateConfig(
		"https://disog-files.s3.amazonaws.com/airplayScanner/airplayScanner.version",
		"https://disog-files.s3.amazonaws.com/airplayScanner/",
		currentVersion,
	)
	return config.CheckForUpdates()
}

func (c *UpdateConfig) CheckForUpdates() error {
	fmt.Println("[!] Checking for updates...")
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: c.Timeout,
	}

	resp, err := client.Get(c.VersionURL)
	if err != nil {
		return errors.Wrap(err, "failed to check for updates")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get version info: %d", resp.StatusCode)
	}

	latestVersion, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read version info")
	}

	latestVersionStr := strings.TrimSpace(string(latestVersion))
	fmt.Printf("[+] Current version: %s\n", c.CurrentVersion)
	fmt.Printf("[+] Latest version: %s\n", latestVersionStr)
	currentParts := strings.Split(c.CurrentVersion, ".")
	latestParts := strings.Split(latestVersionStr, ".")

	if len(currentParts) != 3 || len(latestParts) != 3 {
		return fmt.Errorf("invalid version format: current=%s, latest=%s", c.CurrentVersion, latestVersionStr)
	}

	for i := 0; i < 3; i++ {
		current, err1 := strconv.Atoi(currentParts[i])
		latest, err2 := strconv.Atoi(latestParts[i])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid version number: current=%s, latest=%s", c.CurrentVersion, latestVersionStr)
		}
		if latest > current {
			fmt.Println("[!] New version available, downloading update...")
			execPath, err := os.Executable() // Use executable path so we can update from anywhere.
			if err != nil {
				return errors.Wrap(err, "failed to get executable path")
			}

			tempFile := execPath + ".new" // Create temporary file for the new version
			binaryURL := c.GetPlatformBinaryURL()
			fmt.Printf("[+] Downloading from: %s\n", binaryURL)
			resp, err := client.Get(binaryURL)
			if err != nil {
				return errors.Wrap(err, "failed to download update")
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to download update: %d", resp.StatusCode)
			}

			newBinary, err := io.ReadAll(resp.Body)
			if err != nil {
				return errors.Wrap(err, "failed to read update data")
			}

			err = os.WriteFile(tempFile, newBinary, 0755)
			if err != nil {
				return errors.Wrap(err, "failed to write update file")
			}

			err = os.Rename(tempFile, execPath)
			if err != nil {
				os.Remove(tempFile) // Clean up
				return errors.Wrap(err, "failed to install update")
			}

			fmt.Println("[+] Update completed successfully!")
			os.Exit(0)
		} else if latest < current {
			fmt.Println("[+] You are running a newer version than available.")
			return nil
		}
	}

	fmt.Println("[+] You are running the latest version.")
	return nil
}
