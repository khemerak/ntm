# ntm - Media Downloader

`ntm` is a lightning-fast, zero-friction CLI media downloader written in Go. It acts as a smart wrapper around `yt-dlp`, but removes the headache of managing Python dependencies, keeping binaries updated, or struggling with slow single-threaded download speeds. 

Just give it a URL, and it does the rest.

## Features

- **Zero-Setup Extractor:** `ntm` automatically bootstraps the latest standalone `yt-dlp` binary into your `~/.config/ntm/bin` on first run. No Python environment required.
- **Warp Speed:** Bypasses YouTube bandwidth throttling by automatically splitting downloads into 8 concurrent streams.
- **Smart Defaults:** Defaults to 1080p video quality (to prevent accidental 50GB 8K downloads) and saves directly to your system's `Downloads` folder.
- **Audio Extraction:** One flag to rip high-quality MP3s.
- **Clean UI:** No terminal spam. Real-time, single-line progress bars and human-readable error bubbling.

## Prerequisites

While `ntm` handles the core extractor automatically, you need `ffmpeg` installed on your system to merge video/audio tracks and extract MP3s.

**Arch Linux / CachyOS:**

```bash
sudo pacman -S ffmpeg

```

**Ubuntu / Debian:**

```bash
sudo apt install ffmpeg

```

**macOS:**

```bash
brew install ffmpeg

```

## Installation

Install the latest version globally using the automated install script:

```bash
curl -sL https://raw.githubusercontent.com/khemearak/ntm/main/install.sh | bash

```

## Usage

By default, `ntm` downloads the best video available (up to 1080p) and saves it to your `~/Downloads` directory.

**Basic Video Download:**

```bash
ntm "https://youtu.be/example"

```

**Extract Audio Only (MP3):**

```bash
ntm "https://youtu.be/example" -a

```

**Change Video Quality:**

```bash
# Options: 1080p (default), 720p, best
ntm "https://youtu.be/example" -q 720p

```

**Custom Output Directory:**

```bash
ntm "https://youtu.be/example" -a -o ~/Music

```

**Force Redownload (Ignore Cache):**

```bash
ntm "https://youtu.be/example" -f

```

## Building from Source

If you want to compile the project manually:

```bash
git clone https://github.com/khemerak/ntm.git
cd ntm
go build -ldflags="-s -w" -o ntm ./cmd/ntm
sudo mv ntm /usr/local/bin/

```

