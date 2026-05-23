package extractor

type Metadata struct {
	Title       string
	DurationSec int
	Extractor   string
}

type MediaExtractor interface {
	CanHandle(url string) bool
	ExtractMetadata(url string) (*Metadata, error)
	Download(url string, outputDir string, audioOnly bool, quality string, force bool) error
}
