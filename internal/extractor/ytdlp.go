package extractor

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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

func (e *YTDLPExtractor) CanHandle(url string) bool {
	supportedDomains := []string{
		"youtube.com", "youtu.be",
		"tiktok.com",
		"x.com", "twitter.com",
		"facebook.com", "fb.watch",
		"instagram.com",
	}
	lowerURL := strings.ToLower(url)
	for _, domain := range supportedDomains {
		if strings.Contains(lowerURL, domain) {
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
		return nil, fmt.Errorf("Failed to extract metadata %s", stderr.String())
	}

	var rawData struct {
		Title    string `json:title`
		Duration int    `json:duration`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rawData); err != nil {
		return nil, fmt.Errorf("failed to pharse JSON metadata %v", err)
	}

	return &Metadata{
		Title:       rawData.Title,
		DurationSec: rawData.Duration,
		Extractor:   "yt-dlp",
	}, nil
}

func (e *YTDLPExtractor) Download(url string, outputDir string, audioOnly bool, quality string, force bool) error {
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory %v", err)
	}

	args := []string{
		"--newline",
		"--js-runtimes", "node",
		"--no-playlist",
		"--restrict-filenames",
		//"--no-warnings",
		"-4",
		"--extractor-args", "youtube:player_client=web",
		"-N", "4",
		"-o", fmt.Sprintf("%s/%%(title)s.%%(ext)s", outputDir),
	}

	if e.ffmpegBinDir != "" {
		args = append(args, "--ffmpeg-location", e.ffmpegBinDir)
	}
	if force {
		args = append(args, "--force-overwrites")
	}

	if audioOnly {
		args = append(args, "-f", "bestaudio/best", "-x") //, "--audio-format", "m4a", "--audio-quality", "0")
	} else {
		formatStr := "bestvideo+bestaudio/best"
		if quality == "1080p" {
			formatStr = "bestvideo[height<=1080]+bestaudio/best[height<=1080]/best"
		} else if quality == "720p" {
			formatStr = "bestvideo[height<=720]+bestaudio/best[height<=720]/best"
		}
		args = append(args, "-f", formatStr)
	}

	args = append(args, url)

	cmd := exec.Command(e.binaryPath, args...)

	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start download: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
		writer.Close()
	}()

	progressRe := regexp.MustCompile(`\[download\]\s+(~?\s*[0-9\.]+)%`)
	scanner := bufio.NewScanner(reader)

	spinner := []string{"|", "/", "-", "\\"}
	spinnerIndex := 0

	var isExtracting bool
	var isMerging bool
	var hasPrintedMetadata bool

	barWidth := 40
	fmt.Println()

	for scanner.Scan() {
		line := scanner.Text()

		if !hasPrintedMetadata && strings.Contains(line, "[download] Destination:") {
			fullPath := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
			filename := filepath.Base(fullPath)
			ext := filepath.Ext(filename)
			cleanTitle := strings.TrimSuffix(filename, ext)

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
		} else if strings.Contains(strings.ToLower(line), "error:") {
			parts := strings.SplitN(strings.ToLower(line), "error:", 2)
			cleanErr := strings.TrimSpace(parts[len(parts)-1])
			fmt.Printf("\r\033[K  \033[31m[\033[0m ERROR \033[31m]\033[0m %s\n", cleanErr)
		} else if strings.Contains(line, "ERROR:") || strings.Contains(line, "WARNING:") {
			fmt.Printf("\n  \033[31m[\033[0m ! \033[31m]\033[0m %s\n", line)
		}
	}

	fmt.Println()
	if err := <-errCh; err != nil {
		return fmt.Errorf("process has crashed: %v", err)
	}

	return nil
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
	info, err := os.Stat(binPath)
	if err == nil && info.Size() > 10000000 {
		return binPath, nil
	}
	fmt.Println("Downloading yt-dlp standalone binary (This is a one-time installation.)...")

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	out, err := os.OpenFile(binPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
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
	osKey := ""

	if runtime.GOOS == "windows" {
		ffmpegName += ".exe"
		ffprobeName += ".exe"
		osKey = "win-64"
	} else if runtime.GOOS == "darwin" {
		osKey = "macos-64"
	} else {
		osKey = "linux-64"
	}

	ffmpegPath := filepath.Join(appDir, ffmpegName)
	ffprobePath := filepath.Join(appDir, ffprobeName)

	fi, err1 := os.Stat(ffmpegPath)
	pi, err2 := os.Stat(ffprobePath)
	if err1 == nil && err2 == nil && fi.Size() > 5000000 && pi.Size() > 5000000 {
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

	return appDir, nil
}

func downloadAndExtractZip(url string, destDir string, targetBinary string) error {
	tmpFile, err := os.CreateTemp("", "ffbin-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	resp, err := http.Get(url)
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
