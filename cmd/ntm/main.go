package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ntm/internal/extractor"
)

var (
	outputDir          string
	audioOnly          bool
	quality            string
	force              bool
	cookiesFromBrowser string
	cookiesFile        string
	impersonate        string
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "ntm [url]",
	Short:   "Minimal high-performance media downloader",
	Version: Version,
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetURL := sanitizeURL(args[0])

		fmt.Println("\n  \033[36m┌──────────────────────────────────────────┐\033[0m")
		fmt.Println("  \033[36m│\033[0m                                          \033[36m│\033[0m")
		fmt.Println("  \033[36m│\033[0m          \033[1mNTM : MEDIA DOWNLOADER   \033[0m       \033[36m│\033[0m")
		fmt.Println("  \033[36m│\033[0m                                          \033[36m│\033[0m")
		fmt.Printf("  \033[36m└──────────────────────────────────────────┘\033[0m\n\n")

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

		err = ext.Download(targetURL, extractor.DownloadOptions{
			OutputDir:          outputDir,
			AudioOnly:          audioOnly,
			Quality:            quality,
			Force:              force,
			CookiesFromBrowser: cookiesFromBrowser,
			CookiesFile:        cookiesFile,
			Impersonate:        impersonate,
		})
		if err != nil {
			fmt.Printf("  \033[31m[\033[0m ! \033[31m]\033[0m Download failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n  \033[32m[\033[0m ✓ \033[32m]\033[0m Download completed successfully.\n\n")
	},
}

// sanitizeURL removes playlist and tracking parameters so a single video is
// downloaded instead of an entire playlist or radio mix.
func sanitizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	query := parsed.Query()
	for _, key := range []string{"list", "index", "start_radio", "rv", "si", "pp"} {
		query.Del(key)
	}
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of ntm",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ntm %s\n", Version)
	},
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	defaultDownloadPath := filepath.Join(home, "Downloads")

	rootCmd.Flags().StringVarP(&outputDir, "output", "o", defaultDownloadPath, "Directory to save the downloaded media")
	rootCmd.Flags().BoolVarP(&audioOnly, "audio", "a", false, "Extract audio only (M4A)")
	rootCmd.Flags().StringVarP(&quality, "quality", "q", "1080p", "Video quality: best, 1080p, 720p, or any height")
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite if the file exists")
	rootCmd.Flags().StringVar(&cookiesFromBrowser, "cookies-from-browser", "", "Load cookies from a browser (chrome, edge, firefox, brave, ...)")
	rootCmd.Flags().StringVar(&cookiesFile, "cookies", "", "Path to a Netscape-format cookies.txt file")
	rootCmd.Flags().StringVar(&impersonate, "impersonate", "", "Impersonate a browser/client (for example: chrome, firefox)")
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
