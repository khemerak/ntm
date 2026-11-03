package extractor

type Metadata struct {
	Title       string
	DurationSec int
	Extractor   string
}

// DownloadOptions controls a single download request.
type DownloadOptions struct {
	OutputDir string
	AudioOnly bool
	Quality   string
	Force     bool

	// CookiesFromBrowser is passed to yt-dlp as --cookies-from-browser
	// (for example "chrome", "edge", "firefox", or "chrome:Profile 1").
	CookiesFromBrowser string
	// CookiesFile is a path to a Netscape-format cookies.txt file.
	CookiesFile string
	// Impersonate is passed to yt-dlp as --impersonate (for example "chrome").
	Impersonate string
}

type MediaExtractor interface {
	CanHandle(url string) bool
	ExtractMetadata(url string) (*Metadata, error)
	Download(url string, opts DownloadOptions) error
}
