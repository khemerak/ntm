package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"ntm/internal/extractor"
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

		fmt.Println("\n  \033[36m┌──────────────────────────────────────────┐\033[0m")
		fmt.Println("  \033[36m│\033[0m                                          \033[36m│\033[0m")
		fmt.Println("  \033[36m│\033[0m       \033[1mNTM : MEDIA DOWNLOADER      \033[0m       \033[36m│\033[0m")
		fmt.Println("  \033[36m│\033[0m                                          \033[36m│\033[0m")
		fmt.Println("  \033[36m└──────────────────────────────────────────┘\033[0m\n")

		fmt.Printf("  \033[36m[\033[0m i \033[36m]\033[0m Target  : %s\n", targetURL)
		fmt.Printf("  \033[36m[\033[0m i \033[36m]\033[0m Output  : %s\n", outputDir)

		if audioOnly {
			fmt.Printf("  \033[36m[\033[0m i \033[36m]\033[0m Mode    : Audio Only (MP3)\n\n")
		} else {
			fmt.Printf("  \033[36m[\033[0m i \033[36m]\033[0m Mode    : Video (%s)\n\n", quality)
		}

		ext, err := extractor.NewYTDLPExtractor()
		if err != nil {
			fmt.Printf("  \033[31m[\033[0m ! \033[31m]\033[0m Error: %v\n", err)
			os.Exit(1)
		}

		if !ext.CanHandle(targetURL) {
			fmt.Println("  \033[31m[\033[0m ! \033[31m]\033[0m Error: Unsupported URL format.")
			os.Exit(1)
		}

		fmt.Println("  \033[33m[\033[0m * \033[33m]\033[0m Fetching metadata...")
		meta, err := ext.ExtractMetadata(targetURL)
		if err != nil {
			fmt.Printf("  \033[31m[\033[0m ! \033[31m]\033[0m Warning: Could not fetch metadata.\n")
		} else {
			fmt.Printf("  \033[32m[\033[0m * \033[32m]\033[0m Found   : %s (%ds)\n", meta.Title, meta.DurationSec)
		}

		//fmt.Println("\n  \033[33m[\033[0m * \033[33m]\033[0m Downloading streams...")

		err = ext.Download(targetURL, outputDir, audioOnly, quality, force)
		if err != nil {
			fmt.Printf("\n  \033[31m[\033[0m ! \033[31m]\033[0m Download failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n  \033[32m[\033[0m ✓ \033[32m]\033[0m Download completed successfully.\n")
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
