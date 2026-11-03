# ntm Project Context

## Overview

`ntm` is a Go CLI media downloader that wraps `yt-dlp`. It aims to be a
zero-dependency experience for the user: on first run it downloads a standalone
`yt-dlp` binary plus static `ffmpeg`/`ffprobe` builds into the user's config
directory, then uses yt-dlp to download and merge media.

Supported platforms (per the current allow-list): YouTube, TikTok, X/Twitter,
Facebook, and Instagram.

## Tech Stack

- **Language:** Go (`module ntm`, `go 1.26.3`)
- **CLI framework:** `github.com/spf13/cobra`
- **Download engine:** standalone `yt-dlp` executable (not the Python package)
- **Media tooling:** `ffmpeg` and `ffprobe` from
  `ffbinaries-prebuilt` v6.1

## Repository Layout

```text
cmd/ntm/
  main.go        CLI entrypoint, flags, URL sanitization, banner/UI
  update.go      self-update command and yt-dlp binary update
  main_test.go   URL sanitization tests

internal/extractor/
  extractor.go   Metadata type and MediaExtractor interface
  ytdlp.go       yt-dlp bootstrap, download, progress streaming, ffmpeg bootstrap
  ytdlp_test.go  format selector, URL support, platform key tests

.github/workflows/release.yml  tag-driven multi-platform release builds
install.ps1                    Windows installer
install.sh                     Linux/macOS installer
README.md                      user-facing docs
progress.md                    current status and change log
context.md                     this file
```

## Runtime Flow

1. `cmd/ntm/main.go` parses flags and calls `sanitizeURL`, which removes
   playlist/radio/tracking parameters (`list`, `index`, `start_radio`, `rv`,
   `si`, `pp`).
2. `extractor.NewYTDLPExtractor()`:
   - `ensureYTDLP()` downloads or validates the yt-dlp binary.
   - `ensureFFTools()` downloads or validates ffmpeg/ffprobe.
   - FFmpeg setup failure is a warning, not a fatal error. Downloading still
     works, but separate video/audio streams cannot be merged.
3. `CanHandle()` parses the hostname and accepts the URL only when the host is
   one of the supported domains or a subdomain of one.
4. `Download()` builds the yt-dlp command, streams stdout/stderr through an
   `io.Pipe`, and renders a single-line progress bar.
5. On success yt-dlp has already written the final file into the output
   directory (default: `~/Downloads`).

## Binary Locations

`os.UserConfigDir()` is used for the bootstrap directory:

- Windows: `%APPDATA%\ntm\bin`
- Linux: `~/.config/ntm/bin`
- macOS: `~/Library/Application Support/ntm/bin`

The binaries are `yt-dlp` / `yt-dlp.exe`, `ffmpeg`, and `ffprobe` (with `.exe`
on Windows).

## Important Design Decisions

### Do not force a YouTube player client

An earlier version passed:

```text
--extractor-args youtube:player_client=web
```

That forced the `web` YouTube client, which currently returns only storyboard
images to yt-dlp (the "Only images are available for download" warning). Any
real `-f` selector then failed with "requested format is not available".
`yt-dlp`'s built-in client rotation must be left alone.

### JavaScript runtimes are optional and auto-detected

Recent yt-dlp versions use a JavaScript runtime to solve YouTube challenges.
Only `deno` is enabled by default, so `ntm` detects `deno`, `node`, and `bun`
with `exec.LookPath` and enables every runtime that is actually installed.
Never pass a runtime that does not exist.

### Format selection depends on ffmpeg

When ffmpeg is available, `ntm` asks for separate best video + best audio
streams and falls back to a pre-muxed format:

```text
best:  bv*+ba/b
1080p: bv*[height<=1080]+ba/b[height<=1080]/b
720p:  bv*[height<=720]+ba/b[height<=720]/b
```

Without ffmpeg, only pre-muxed formats are selected (`b`, `b[height<=N]/b`)
because there is no way to merge separate streams.

### Atomic bootstrap downloads

Bootstrap downloads are written to a temporary file in the destination
directory, validated by size, then renamed into place. HTTP status codes are
checked so an error page is never saved as an executable.

### Audio output

Audio mode uses `bestaudio/best` and, when ffmpeg is available,
`-x --audio-format m4a --audio-quality 0`, matching the CLI's "Audio Only (M4A)"
label.

## Build, Test, Release

```bash
go build -o ntm ./cmd/ntm
go vet ./...
go test ./...
gofmt -w cmd internal
```

Release builds are triggered by pushing a `v*` tag. The workflow produces:

```text
ntm-linux-amd64
ntm-linux-arm64
ntm-darwin-amd64
ntm-darwin-arm64
ntm-windows-amd64.exe
```

There is intentionally no `ntm-windows-arm64.exe`; the install script and
`ntm update` use the amd64 build on Windows ARM64 via x64 emulation.
