package extractor

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	minYTDLPSize  = 10 * 1024 * 1024
	minFFmpegSize = 5 * 1024 * 1024
)

type YTDLPExtractor struct {
	binaryPath   string
	ffmpegBinDir string
}

func NewYTDLPExtractor() (*YTDLPExtractor, error) {
	binPath, err := ensureYTDLP()
	if err != nil {
		return nil, fmt.Errorf("failed to setup yt-dlp: %v", err)
	}

	ffmpegDir, err := ensureFFTools()
	if err != nil {
		fmt.Printf("\n  \033[33m[\033[0m ! \033[33m]\033[0m Warning: Internal FFmpeg setup failed. Audio extraction may not work: %v\n", err)
	}

	return &YTDLPExtractor{
		binaryPath:   binPath,
		ffmpegBinDir: ffmpegDir,
	}, nil

}

func (e *YTDLPExtractor) CanHandle(rawURL string) bool {
	supportedDomains := []string{
		"youtube.com", "youtu.be",
		"tiktok.com",
		"x.com", "twitter.com",
		"facebook.com", "fb.watch",
		"instagram.com",
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	for _, domain := range supportedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func (e *YTDLPExtractor) ExtractMetadata(url string) (*Metadata, error) {
	cmd := exec.Command(e.binaryPath, "--dump-json", "--no-playlist", url)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %s", strings.TrimSpace(stderr.String()))
	}

	var rawData struct {
		Title    string `json:"title"`
		Duration int    `json:"duration"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON metadata: %v", err)
	}

	return &Metadata{
		Title:       rawData.Title,
		DurationSec: rawData.Duration,
		Extractor:   "yt-dlp",
	}, nil
}

// buildFormatSelector picks a yt-dlp format expression for the requested quality.
// Separate video/audio streams can only be merged when ffmpeg is available, so
// without ffmpeg we fall back to pre-muxed formats.
func buildFormatSelector(quality string, hasFFmpeg bool) string {
	height := 0
	if quality != "best" {
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(strings.TrimSpace(quality)), "p")); err == nil && n > 0 {
			height = n
		}
	}

	if height == 0 {
		if hasFFmpeg {
			return "bv*+ba/b"
		}
		return "b"
	}

	if hasFFmpeg {
		return fmt.Sprintf("bv*[height<=%d]+ba/b[height<=%d]/b", height, height)
	}
	return fmt.Sprintf("b[height<=%d]/b", height)
}

// detectJSRuntimes returns the JavaScript runtimes that yt-dlp can use to solve
// YouTube challenges. Only deno is enabled by default, so node/bun must be
// explicitly enabled when they are the runtimes installed on the machine.
func detectJSRuntimes() []string {
	var runtimes []string
	for _, name := range []string{"deno", "node", "bun"} {
		if _, err := exec.LookPath(name); err == nil {
			runtimes = append(runtimes, name)
		}
	}
	return runtimes
}

func (e *YTDLPExtractor) Download(url string, opts DownloadOptions) error {
	if err := os.MkdirAll(opts.OutputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	args := []string{
		"--newline",
		"--no-playlist",
		"--restrict-filenames",
		"--retries", "10",
		"--fragment-retries", "10",
		"--extractor-retries", "3",
		"--socket-timeout", "30",
		"-N", "4",
		"-o", filepath.Join(opts.OutputDir, "%(title)s.%(ext)s"),
	}

	// Only enable runtimes that actually exist. Forcing a missing runtime (for
	// example "node") leaves yt-dlp unable to solve YouTube challenges.
	jsRuntimes := detectJSRuntimes()
	for _, runtimeName := range jsRuntimes {
		args = append(args, "--js-runtimes", runtimeName)
	}
	if len(jsRuntimes) == 0 && strings.Contains(strings.ToLower(url), "youtube") {
		fmt.Println("  \033[33m[\033[0m ! \033[33m]\033[0m WARNING: No JavaScript runtime (deno/node/bun) found. Some YouTube formats may be unavailable.")
	}

	if opts.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
	}
	if opts.CookiesFile != "" {
		info, err := os.Stat(opts.CookiesFile)
		if err != nil {
			return fmt.Errorf("cookies file not found: %v", err)
		}
		if info.IsDir() {
			return fmt.Errorf("cookies path is a directory, expected a file: %s", opts.CookiesFile)
		}
		args = append(args, "--cookies", opts.CookiesFile)
	}
	if opts.Impersonate != "" {
		args = append(args, "--impersonate", opts.Impersonate)
	}

	hasFFmpeg := e.ffmpegBinDir != ""
	if hasFFmpeg {
		args = append(args, "--ffmpeg-location", e.ffmpegBinDir)
	}
	if opts.Force {
		args = append(args, "--force-overwrites")
	}

	if opts.AudioOnly {
		args = append(args, "-f", "bestaudio/best")
		if hasFFmpeg {
			args = append(args, "-x", "--audio-format", "m4a", "--audio-quality", "0")
		}
	} else {
		args = append(args, "-f", buildFormatSelector(opts.Quality, hasFFmpeg))
	}

	args = append(args, url)

	cmd := exec.Command(e.binaryPath, args...)

	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		writer.Close()
		reader.Close()
		return fmt.Errorf("failed to start download: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
		writer.Close()
	}()

	progressRe := regexp.MustCompile(`\[download\]\s+(~?\s*[0-9\.]+)%`)
	formatIDRe := regexp.MustCompile(`\.f[0-9]+$`)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	spinner := []string{"|", "/", "-", "\\"}
	spinnerIndex := 0

	var isExtracting bool
	var isMerging bool
	var hasPrintedMetadata bool
	var lastError string

	barWidth := 40
	fmt.Println()

	for scanner.Scan() {
		line := scanner.Text()

		if !hasPrintedMetadata && strings.Contains(line, "[download] Destination:") {
			fullPath := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
			filename := filepath.Base(fullPath)
			ext := filepath.Ext(filename)
			cleanTitle := strings.TrimSuffix(filename, ext)
			cleanTitle = formatIDRe.ReplaceAllString(cleanTitle, "")

			fmt.Printf("\r\033[K  \033[32m[\033[0m * \033[32m]\033[0m Found   : %s\n", cleanTitle)
			hasPrintedMetadata = true
		}

		if match := progressRe.FindStringSubmatch(line); len(match) > 1 {
			cleanPercent := strings.TrimSpace(strings.ReplaceAll(match[1], "~", ""))
			percentFloat, err := strconv.ParseFloat(cleanPercent, 64)

			if err == nil {
				completedSteps := int((percentFloat / 100.0) * float64(barWidth))
				if completedSteps > barWidth {
					completedSteps = barWidth
				}
				uncompletedSteps := barWidth - completedSteps

				bar := strings.Repeat("█", completedSteps) + strings.Repeat("░", uncompletedSteps)

				s := spinner[spinnerIndex]
				spinnerIndex = (spinnerIndex + 1) % len(spinner)

				fmt.Printf("\r\033[K  \033[36m[\033[0m %s \033[36m]\033[0m Downloading media \033[36m[%s]\033[0m %5s%%", s, bar, cleanPercent)
			}
		} else if strings.Contains(line, "ERROR:") {
			idx := strings.Index(line, "ERROR:")
			lastError = strings.TrimSpace(line[idx+len("ERROR:"):])
			fmt.Printf("\r\033[K  \033[31m[\033[0m ERROR \033[31m]\033[0m %s\n", lastError)
		} else if strings.Contains(line, "WARNING:") {
			idx := strings.Index(line, "WARNING:")
			warning := strings.TrimSpace(line[idx+len("WARNING:"):])
			fmt.Printf("\r\033[K  \033[33m[\033[0m ! \033[33m]\033[0m WARNING: %s\n", warning)
		} else if strings.Contains(line, "[ExtractAudio]") {
			if !isExtracting {
				fmt.Printf("\n  \033[33m[\033[0m * \033[33m]\033[0m Extracting audio track...")
				isExtracting = true
			}
		} else if strings.Contains(line, "[Merger]") {
			if !isMerging {
				fmt.Printf("\n\n  \033[33m[\033[0m * \033[33m]\033[0m Merging video and audio tracks...")
				isMerging = true
			}
		} else if strings.Contains(line, "has already been downloaded") {
			fmt.Printf("\n  \033[33m[\033[0m ! \033[33m]\033[0m File already exists. Pass --force to overwrite.")
		}
	}

	// If the scanner stopped early (for example on an oversized line), keep
	// draining the pipe so the child process never blocks on a full buffer.
	if err := scanner.Err(); err != nil {
		_, _ = io.Copy(io.Discard, reader)
	}

	fmt.Println()
	if err := <-errCh; err != nil {
		if lastError != "" {
			if isAuthError(lastError) {
				printAuthHint(url, opts)
			} else if isCookieReadError(lastError) {
				printCookieReadHint(url)
			}
			return fmt.Errorf("%s", lastError)
		}
		return fmt.Errorf("yt-dlp failed: %v", err)
	}

	return nil
}

// isAuthError reports whether yt-dlp was blocked by YouTube's login/bot check
// or rate limiting rather than by an invalid format or a missing video.
func isAuthError(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "sign in to confirm") ||
		strings.Contains(lower, "not a bot") ||
		strings.Contains(lower, "use --cookies") ||
		strings.Contains(lower, "http error 429") ||
		strings.Contains(lower, "too many requests")
}

// isCookieReadError reports whether yt-dlp could not read the browser's cookie
// database (for example Chrome/Edge app-bound encryption on Windows).
func isCookieReadError(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "failed to decrypt with dpapi") ||
		strings.Contains(lower, "could not copy chrome cookie database") ||
		strings.Contains(lower, "could not find cookies") ||
		strings.Contains(lower, "cookie database")
}

func printAuthHint(url string, opts DownloadOptions) {
	fmt.Println()
	if opts.CookiesFromBrowser == "" && opts.CookiesFile == "" {
		fmt.Println("  \033[36m[\033[0m i \033[36m]\033[0m YouTube is asking for authentication (bot check / rate limit).")
		fmt.Println("        Use cookies from a browser where you are signed in to YouTube:")
		fmt.Printf("          ntm %q --cookies-from-browser edge\n", url)
		fmt.Println("        Or export cookies.txt and use:")
		fmt.Printf("          ntm %q --cookies C:\\path\\to\\cookies.txt\n", url)
		return
	}

	fmt.Println("  \033[36m[\033[0m i \033[36m]\033[0m Cookies were supplied, but YouTube still rejected the request.")
	fmt.Println("        Make sure the selected browser profile is signed in, or refresh cookies.txt.")
}

func printCookieReadHint(url string) {
	fmt.Println()
	fmt.Println("  \033[36m[\033[0m i \033[36m]\033[0m Could not read the browser cookie database (common with recent Chrome/Edge on Windows).")
	fmt.Println("        Export a Netscape-format cookies.txt with a browser extension, then retry:")
	fmt.Printf("          ntm %q --cookies C:\\path\\to\\cookies.txt\n", url)
}

func ensureYTDLP() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "ntm", "bin")
	if err := os.MkdirAll(appDir, os.ModePerm); err != nil {
		return "", err
	}

	binName := "yt-dlp"
	downloadFilename := "yt-dlp_linux"

	if runtime.GOOS == "windows" {
		binName = "yt-dlp.exe"
		downloadFilename = "yt-dlp.exe"
	} else if runtime.GOOS == "darwin" {
		downloadFilename = "yt-dlp_macos"
	}

	downloadURL := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/" + downloadFilename
	binPath := filepath.Join(appDir, binName)
	if info, err := os.Stat(binPath); err == nil && info.Size() >= minYTDLPSize {
		return binPath, nil
	}

	// Remove a missing/partial/corrupt binary so a retry starts cleanly.
	_ = os.Remove(binPath)

	fmt.Println("Downloading yt-dlp standalone binary (This is a one-time installation.)...")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad HTTP status while downloading yt-dlp: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp(appDir, "yt-dlp-download-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		return "", err
	}
	if info.Size() < minYTDLPSize {
		return "", fmt.Errorf("downloaded yt-dlp binary is unexpectedly small (%d bytes)", info.Size())
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			return "", err
		}
	}

	if err := os.Rename(tmpPath, binPath); err != nil {
		return "", err
	}

	return binPath, nil
}

func (e *YTDLPExtractor) GetBinaryPath() string {
	return e.binaryPath
}

func ensureFFTools() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(configDir, "ntm", "bin")
	if err := os.MkdirAll(appDir, os.ModePerm); err != nil {
		return "", err
	}

	ffmpegName := "ffmpeg"
	ffprobeName := "ffprobe"
	if runtime.GOOS == "windows" {
		ffmpegName += ".exe"
		ffprobeName += ".exe"
	}

	osKey, err := ffbinariesPlatformKey()
	if err != nil {
		return "", err
	}

	ffmpegPath := filepath.Join(appDir, ffmpegName)
	ffprobePath := filepath.Join(appDir, ffprobeName)

	fi, err1 := os.Stat(ffmpegPath)
	pi, err2 := os.Stat(ffprobePath)
	if err1 == nil && err2 == nil && fi.Size() >= minFFmpegSize && pi.Size() >= minFFmpegSize {
		return appDir, nil
	}

	fmt.Println("  \033[36m[\033[0m * \033[36m]\033[0m Bootstrapping internal FFmpeg tools(This is a one-time installation of FFmpeg.)...")

	baseURL := "https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download/v6.1"
	ffmpegURL := fmt.Sprintf("%s/ffmpeg-6.1-%s.zip", baseURL, osKey)
	ffprobeURL := fmt.Sprintf("%s/ffprobe-6.1-%s.zip", baseURL, osKey)

	if err := downloadAndExtractZip(ffmpegURL, appDir, ffmpegName); err != nil {
		return "", fmt.Errorf("ffmpeg download failed: %v", err)
	}
	if err := downloadAndExtractZip(ffprobeURL, appDir, ffprobeName); err != nil {
		return "", fmt.Errorf("ffprobe download failed: %v", err)
	}

	for _, path := range []string{ffmpegPath, ffprobePath} {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.Size() < minFFmpegSize {
			return "", fmt.Errorf("downloaded %s is unexpectedly small (%d bytes)", filepath.Base(path), info.Size())
		}
	}

	return appDir, nil
}

// ffbinariesPlatformKey maps the current Go platform to the artifact naming
// used by the ffbinaries-prebuilt releases.
func ffbinariesPlatformKey() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// Only 64-bit builds are published. ARM64 Windows can run the x64
		// build through emulation.
		if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
			return "win-64", nil
		}
	case "darwin":
		// ffbinaries only publishes an x86_64 macOS build; Apple Silicon runs
		// it through Rosetta 2.
		return "macos-64", nil
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "linux-64", nil
		case "arm64":
			return "linux-arm-64", nil
		case "arm":
			return "linux-armhf-32", nil
		case "386":
			return "linux-32", nil
		}
	}

	return "", fmt.Errorf("unsupported platform for FFmpeg bootstrap: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func downloadAndExtractZip(url string, destDir string, targetBinary string) error {
	tmpFile, err := os.CreateTemp("", "ffbin-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	resp, err := (&http.Client{Timeout: 15 * time.Minute}).Get(url)
	if err != nil {
		tmpFile.Close()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("bad HTTP status: %d %s", resp.StatusCode, resp.Status)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		baseName := filepath.Base(f.Name)
		cleanBase := strings.TrimSuffix(baseName, ".exe")
		cleanTarget := strings.TrimSuffix(targetBinary, ".exe")

		if cleanBase == cleanTarget {
			rc, err := f.Open()
			if err != nil {
				return err
			}

			destPath := filepath.Join(destDir, targetBinary)
			out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				return err
			}

			_, copyErr := io.Copy(out, rc)
			out.Close()
			rc.Close()
			return copyErr
		}
	}
	return fmt.Errorf("Could not find %s inside the downloaded archive.", targetBinary)
}
