package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"ntm/internal/extractor"
	"os"
	"path/filepath"
	"strings"
	//"net/url"
)

var (
	outputDir string
	audioOnly bool
	quality   string
	force     bool
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "ntm [url]",
	Short:   "Minimal high-performance media downloader",
	Version: Version,
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetURL := args[0]

		//		parsedURL, err := url.Parse(targetURL)
		//		if err != nil {
		//			q := parsedURL.Query()
		//			q.Del("list")
		//			q.Del("start_radio")
		//			q.Del("rv")
		//			q.Del("index")
		//			parsedURL.RawQuery = q.Encode()
		//			targetURL = parsedURL.String()
		//		}

		if idx := strings.Index(targetURL, "&list="); idx != -1 {
			targetURL = targetURL[:idx]
		}
		if idx := strings.Index(targetURL, "?list="); idx != -1 {
			targetURL = targetURL[:idx]
		}
		if idx := strings.Index(targetURL, "&start_radio="); idx != -1 {
			targetURL = targetURL[:idx]
		}

		fmt.Println("\n  \033[36m┌──────────────────────────────────────────┐\033[0m")
		fmt.Println("  \033[36m│\033[0m                                          \033[36m│\033[0m")
		fmt.Println("  \033[36m│\033[0m          \033[1mNTM : MEDIA DOWNLOADER   \033[0m       \033[36m│\033[0m")
		fmt.Println("  \033[36m│\033[0m                                          \033[36m│\033[0m")
		fmt.Println("  \033[36m└──────────────────────────────────────────┘\033[0m\n")

		fmt.Printf("  \033[36m[\033[0m i \033[36m]\033[0m Target  : %s\n", targetURL)
		fmt.Printf("  \033[36m[\033[0m i \033[36m]\033[0m Output  : %s\n", outputDir)

		if audioOnly {
			fmt.Printf("  \033[36m[\033[0m i \033[36m]\033[0m Mode    : Audio Only (M4A)\n\n")
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

		fmt.Println("  \033[33m[\033[0m * \033[33m]\033[0m Connecting to stream...")

		err = ext.Download(targetURL, outputDir, audioOnly, quality, force)
		if err != nil {
			fmt.Printf("  \033[31m[\033[0m ! \033[31m]\033[0m Download failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n  \033[32m[\033[0m ✓ \033[32m]\033[0m Download completed successfully.\n")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of ntm",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ntm %s", Version)
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
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
