package extractor

import (
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
	binaryPath string
}

func NewYTDLPExtractor() (*YTDLPExtractor, error) {
	binPath, err := ensureYTDLP()
	if err != nil {
		return nil, fmt.Errorf("failed to setup yt-dlp: %v", err)
	}
	return &YTDLPExtractor{binaryPath: binPath}, nil
}

func (e *YTDLPExtractor) CanHandle(url string) bool {
	return strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be")
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
	if force {
		args = append(args, "--force-overwrites")
	}

	if audioOnly {
		args = append(args, "-f", "bestaudio/best", "-x") //, "--audio-format", "m4a", "--audio-quality", "0")
	} else {
		formatStr := "bestvideo+bestaudio/best"
		if quality == "1080p" {
			formatStr = "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
		} else if quality == "720p" {
			formatStr = "bestvideo[height<=720]+bestaudio/best[height<=720]"
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
	fmt.Println("Downloading yt-dlp standalone binary...")

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
