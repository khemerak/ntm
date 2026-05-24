package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ntm/internal/extractor"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	//"strings"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update ntm to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("  \033[36m[\033[0m * \033[36m]\033[0m Checking for updates...")
		repo := "khemerak/ntm"
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

		resp, err := http.Get(apiURL)
		if err != nil {
			fmt.Printf("  \033[31m[\033[0m ! \033[31m]\033[0m Failed to connectto GitHub: %v\n", err)
			return
		}
		defer resp.Body.Close()

		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			fmt.Printf("  \033[31m[\033[0m ! \033[31m]\033[0m Failed to parse GitHub response: %v\n", err)
			return
		}

		if release.TagName == Version {
			fmt.Printf("  \033[32m[\033[0m + \033[32m]\033[0m You are already running the latest version (%s)\n", Version)
			return
		}

		fmt.Printf("  \033[33m[\033[0m * \033[33m]\033[0m New version found: %s (Current: %s)\n", release.TagName, Version)
		goos := runtime.GOOS
		goarch := runtime.GOARCH
		if goarch == "amd64" {

		} else if goarch == "arm64" {

		}

		binaryName := fmt.Sprintf("ntm-%s-%s", goos, goarch)
		if goos == "windows" {
			binaryName += ".exe"
		}

		downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, release.TagName, binaryName)

		if err := executeUpdate(downloadURL); err != nil {
			fmt.Printf("\n  \033[31m[\033[0m ! \033[31m]\033[0m Update failed: %v\n", err)
			fmt.Println("  \033[33m[\033[0m i \033[33m]\033[0m Try running the update command with sudo if you encounter permission errors.")
			return
		}
		fmt.Printf("\n  \033[32m[\033[0m + \033[32m]\033[0m Successfully updated to %s!\n", release.TagName)

		// Update YT-DLP
		fmt.Println("  \033[32m[\033[0m * \033[36m]\033[0m Checking for yt-dlp standalone updates...")
		ext, err := extractor.NewYTDLPExtractor()
		if err != nil {
			fmt.Printf("  \033[31m[\033[0m ! \033[31m]\033[0m Could not locate yt-dlp binary to update: %v\n", err)
			return
		}
		cmdYtdlp := exec.Command(ext.GetBinaryPath(), "--update")
		if err := cmdYtdlp.Run(); err != nil {
			fmt.Println("  \033[33m[\033[0m ! \033[33m]\033[0m yt-dlp update skipped or already up to date.")
		} else {
			fmt.Println("  \033[32m[\033[0m + \033[32m]\033[0m yt-dlp has been updated to the latest version.")
		}
	},
}

func executeUpdate(downloadURL string) error {
	fmt.Printf("  \033[36m[\033[0m > \033[36m]\033[0m Downloading %s...\n", downloadURL)

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(exePath)
	tmpFile, err := os.CreateTemp(dir, "ntm-update-*")
	if err != nil {
		return fmt.Errorf("Could not create temo file (do you need sudo?): %v", err)
	}

	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := http.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()
	if err := os.Chmod(tmpPath, 0755); err != nil {
		if runtime.GOOS != "windows" {
			return err
		}
	}
	if runtime.GOOS == "windows" {
		oldPath := exePath + "old"
		os.Remove(oldPath)
		if err := os.Rename(exePath, oldPath); err != nil {
			return fmt.Errorf("failed to mvoe running executable: %v", err)
		}
	}
	return os.Rename(tmpPath, exePath)
}
