# ntm Progress

## Current Status

**Working.** The reported download failure is fixed, the project builds
cleanly, `go vet` passes, and the unit tests pass. Real downloads were verified
against YouTube:

- video: 144p download, stream merge, and output file written
- audio: M4A extraction written successfully

Last updated: 2026-11-03

## Reported Issue

Running:

```powershell
ntm "https://youtu.be/uj80SSc40Qk" -q best
```

failed with:

```text
WARNING: Only images are available for download. use --list-formats to see them
ERROR: [youtube] ...: requested format is not available
Download failed: process has crashed: exit status 1
```

### Root Cause

`Download()` passed `--extractor-args youtube:player_client=web` to yt-dlp.
That pinned YouTube's `web` client, which currently exposes only storyboard
images (`mhtml`) instead of real media formats. Because the requested format
selector (`bestvideo+bestaudio/best`) could not match an image format, yt-dlp
exited with "requested format is not available".

This was reproduced with the installed yt-dlp binary:

```text
# with the forced client -> only sb0..sb3 storyboard formats
yt-dlp --extractor-args "youtube:player_client=web" -F <url>

# without it -> normal video/audio formats
yt-dlp -F <url>
```

## Fixes Applied

### `internal/extractor/ytdlp.go`

- Removed the hardcoded `--extractor-args youtube:player_client=web`, letting
  yt-dlp use its own working client rotation.
- Removed the unconditional `--js-runtimes node` and replaced it with
  `detectJSRuntimes()`, which enables only installed runtimes (`deno`, `node`,
  `bun`). A warning is printed when no runtime is available for YouTube.
- Replaced the format expressions with merge-safe selectors:
  `bv*+ba/b`, `bv*[height<=N]+ba/b[height<=N]/b`, and the pre-muxed fallbacks
  `b` / `b[height<=N]/b` when ffmpeg is unavailable.
- Added support for arbitrary quality heights such as `480p` (not just
  `best`/`1080p`/`720p`).
- Made audio mode consistent with the UI: M4A via
  `-x --audio-format m4a --audio-quality 0`. Without ffmpeg it downloads the
  best audio stream without attempting extraction.
- Fixed error output so yt-dlp's actual `ERROR:` text is shown instead of the
  generic "process has crashed: exit status 1". Error text is no longer
  lowercased.
- Warnings are now yellow rather than red.
- Increased the scanner buffer and drain the pipe if scanning stops early, so
  long yt-dlp lines cannot deadlock the process.
- Removed forced IPv4 (`-4`).
- Stripped yt-dlp format IDs from the displayed title
  (`Me_at_the_zoo.f278` -> `Me_at_the_zoo`).
- Fixed `ExtractMetadata` JSON struct tags (`json:"title"` / `json:"duration"`).
- `CanHandle` now parses the hostname, so unrelated domains containing
  `x.com` (for example `linux.com`) are rejected and real subdomains are
  accepted.
- `ensureYTDLP` now:
  - checks the HTTP status code
  - downloads to a temporary file and renames atomically
  - validates the binary size before installing
  - removes partial/corrupt binaries before retrying
- `ensureFFTools` now maps `GOARCH` to the correct ffbinaries artifact
  (`linux-64`, `linux-arm-64`, `linux-armhf-32`, `linux-32`, `macos-64`,
  `win-64`) and validates extracted binary sizes.
- HTTP clients use explicit timeouts for bootstrap downloads.

### `cmd/ntm/main.go`

- Replaced the manual query-string truncation with a `net/url`-based
  `sanitizeURL` that removes `list`, `index`, `start_radio`, `rv`, `si`, and
  `pp`, and adds `https://` when the scheme is missing.
- Added a trailing newline to `ntm version` output.
- Fixed `go vet` warnings caused by `fmt.Println` strings ending in `\n`.

### `cmd/ntm/update.go`

- Added an HTTP status check when querying the GitHub releases API.
- Removed dead `amd64`/`arm64` if-blocks.
- Maps Windows ARM64 to the published `windows-amd64` asset.
- Rolls back the original executable if the Windows replacement rename fails.
- Fixed typos in error messages.

### Installers / Docs

- `install.ps1`: handles `PROCESSOR_ARCHITECTURE`/`PROCESSOR_ARCHITEW6432`
  correctly and uses the only published Windows asset (amd64), including on
  ARM64 via emulation.
- `install.sh`: uses `curl -fsSL` so an HTTP error is not installed as a
  binary, and uses `mktemp` with cleanup.
- `README.md`: audio output is documented as M4A and arbitrary quality heights
  are documented.
- Added `context.md` (architecture and design context) and this file.

### Tests Added

- `internal/extractor/ytdlp_test.go`
  - format selector behavior with and without ffmpeg
  - hostname allow-list behavior
  - ffbinaries platform key
- `cmd/ntm/main_test.go`
  - URL sanitization (playlist/radio/share parameters and missing scheme)

## Verification

```text
gofmt -w cmd internal   # clean
go vet ./...            # clean
go test ./...           # ok ntm/cmd/ntm, ok ntm/internal/extractor
```

Manual runs:

```powershell
go build -o "$env:TEMP\ntm-fixed.exe" ./cmd/ntm
& "$env:TEMP\ntm-fixed.exe" "https://youtu.be/jNQXAC9IVRw?si=test" -q 144p -o "$env:TEMP\ntm-download-test"
# -> Me_at_the_zoo.webm, exit 0

& "$env:TEMP\ntm-fixed.exe" "https://youtu.be/jNQXAC9IVRw" -a -o "$env:TEMP\ntm-download-test-audio"
# -> Me_at_the_zoo.m4a, exit 0
```

The original screenshot URL now reports the real upstream problem (in this
environment the video itself is unavailable), instead of a misleading
"requested format is not available" error.

## Known Limitations / Follow-ups

- `macos-arm64` still downloads the x86_64 ffmpeg build; Rosetta 2 is required.
- Windows ARM64 uses the amd64 executable through emulation because the release
  workflow does not build a native ARM64 Windows binary.
- Bootstrap downloads have no resume/retry logic; a failed download is removed
  and retried on the next run.
- `sanitizeURL` removes `list` even from `/playlist` URLs, which leaves an
  unsupported playlist path. A clearer "playlist URLs are not supported"
  message would be a good follow-up.
- The test suite covers pure helpers only. Network/bootstrap paths are verified
  manually.
- `ntm update` requires a reachable GitHub API and may need elevated
  permissions when the executable lives in a protected directory.
