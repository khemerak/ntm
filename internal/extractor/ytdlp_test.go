package extractor

import (
	"runtime"
	"testing"
)

func TestBuildFormatSelector(t *testing.T) {
	tests := []struct {
		name      string
		quality   string
		hasFFmpeg bool
		want      string
	}{
		{"best with ffmpeg", "best", true, "bv*+ba/b"},
		{"best without ffmpeg", "best", false, "b"},
		{"1080p with ffmpeg", "1080p", true, "bv*[height<=1080]+ba/b[height<=1080]/b"},
		{"1080p without ffmpeg", "1080p", false, "b[height<=1080]/b"},
		{"720p with ffmpeg", "720p", true, "bv*[height<=720]+ba/b[height<=720]/b"},
		{"720p without ffmpeg", "720p", false, "b[height<=720]/b"},
		{"numeric quality", "480", true, "bv*[height<=480]+ba/b[height<=480]/b"},
		{"unknown quality falls back to best", "potato", true, "bv*+ba/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildFormatSelector(tt.quality, tt.hasFFmpeg); got != tt.want {
				t.Fatalf("buildFormatSelector(%q, %v) = %q, want %q", tt.quality, tt.hasFFmpeg, got, tt.want)
			}
		})
	}
}

func TestCanHandle(t *testing.T) {
	extractor := &YTDLPExtractor{}

	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.youtube.com/watch?v=abc", true},
		{"https://youtu.be/abc", true},
		{"https://music.youtube.com/watch?v=abc", true},
		{"https://www.tiktok.com/@user/video/123", true},
		{"https://twitter.com/user/status/123", true},
		{"https://www.instagram.com/p/abc", true},
		{"https://linux.com/watch?v=abc", false},
		{"https://example.com/watch?v=abc", false},
		{"not a url", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := extractor.CanHandle(tt.url); got != tt.want {
			t.Errorf("CanHandle(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		message string
		want    bool
	}{
		{"Sign in to confirm you're not a bot", true},
		{"HTTP Error 429: Too Many Requests", true},
		{"requested format is not available", false},
		{"This video is unavailable", false},
	}

	for _, tt := range tests {
		if got := isAuthError(tt.message); got != tt.want {
			t.Errorf("isAuthError(%q) = %v, want %v", tt.message, got, tt.want)
		}
	}
}

func TestFFBinariesPlatformKey(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("unsupported test platform")
	}

	key, err := ffbinariesPlatformKey()
	if err != nil {
		t.Fatalf("ffbinariesPlatformKey() returned an error: %v", err)
	}
	if key == "" {
		t.Fatal("ffbinariesPlatformKey() returned an empty key")
	}
}
