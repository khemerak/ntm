package main

import (
	"fmt"
	"github.com/khemerak/ntm/internal/extractor"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

var (
	outputDir string
	audioOnly bool
	quality   string
	force     bool
)

var rootCmd = &cobra.Command{
	Use:   "ntm [url]",
	Short: "Minimal high-performance media downloader",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetURL := args[0]

		fmt.Printf("Analyzing target: %s\n", targetURL)

		ext, err := extractor.NewYTDLPExtractor()
		if err != nil {
			fmt.Printf("Error : %v", err)
			os.Exit(1)
		}
		if !ext.CanHandle(targetURL) {
			fmt.Println("Error: Unsupported URL. Currently supporting YouTube formats.")
			os.Exit(1)
		}

		fmt.Println("Fetching metadata...")
		meta, err := ext.ExtractMetadata(targetURL)
		if err != nil {
			fmt.Printf("Warning: Could not fetch metadata %v.", err)
		} else {
			fmt.Printf("Found: %s (%dseconds)\n", meta.Title, meta.DurationSec)
		}
		fmt.Printf("Initializing Download to: %s\n", outputDir)
		if audioOnly {
			fmt.Println("Mode: MP3 Download")
		}
		err = ext.Download(targetURL, outputDir, audioOnly, quality, force)
		if err != nil {
			fmt.Printf("Download Failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\n✓ Download completed successfully!")
	},
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	defaultDownloadPath := filepath.Join(home, "Downloads")

	rootCmd.Flags().StringVarP(&outputDir, "output", "o", defaultDownloadPath, "Directory to save the downloaded media")
	rootCmd.Flags().BoolVarP(&audioOnly, "audio", "a", false, "Extract audio only (mp3)")
	rootCmd.Flags().StringVarP(&quality, "quality", "q", "1080p", "Video quality: best, 1080p, 720p")
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite of file exists")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
