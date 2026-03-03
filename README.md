# Criver

A TUI tool for browsing and installing [chromedriver](https://chromedriver.chromium.org/) binaries.

Criver fetches the latest versions from [Chrome for Testing](https://googlechromelabs.github.io/chrome-for-testing/), detects your installed Chrome/Chromium to suggest a matching version, and installs the selected binary to `/opt/bro/bin`.

## Install

```sh
go install github.com/adis/criver/cmd/criver@latest
```

Or build from source:

```sh
git clone https://github.com/adis/criver.git
cd criver
go build -o criver ./cmd/criver/
```

## Usage

```sh
./criver
```

This opens an interactive TUI that:

1. Detects installed Chrome and Chromium versions
2. Fetches the latest 3 major chromedriver releases
3. Shows a list with recommended versions highlighted (matching your installed browser)
4. Downloads, extracts, and installs the selected chromedriver to `/opt/bro/bin/chromedriver`
5. Optionally appends `/opt/bro/bin` to your shell's PATH (`~/.zshrc`, `~/.bashrc`, etc.)

### Controls

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate version list |
| `enter` | Select version / confirm prompt |
| `y` / `n` | Choose yes/no on PATH prompt |
| `←` / `→` / `tab` | Toggle yes/no |
| `q` / `ctrl+c` | Quit |

## Supported Platforms

| OS | Arch | Platform string |
|---|---|---|
| macOS | ARM64 | `mac-arm64` |
| macOS | x86_64 | `mac-x64` |
| Linux | x86_64 | `linux64` |
| Windows | x86_64 | `win64` |
| Windows | x86 | `win32` |

## Requirements

- Go 1.21+
- Internet access (to fetch version list and download binaries)
- `sudo` access may be required to write to `/opt/bro/bin`
