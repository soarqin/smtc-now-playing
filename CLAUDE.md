# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**smtc-now-playing** is a Windows desktop application that displays "Now Playing" information from the Windows System Media Transport Controls (SMTC) as a web page. It captures media playback information (artist, title, album art, progress) and exposes it via a local web server with WebSocket updates.

## Architecture

The application consists of three layers:

1. **C++ DLL (`c/`)** - Windows WinRT layer that interfaces with `GlobalSystemMediaTransportControlsSessionManager` to capture media data
2. **Go application** - Main executable that orchestrates the system:
   - `smtc.go` - FFI bindings to `smtc.dll` using `purego`
   - `webserver.go` - HTTP/WebSocket server for serving themes and real-time updates
   - `ui.go` - Native Windows GUI using `windigo` (system tray, settings dialog)
   - `config.go` - Configuration management (portable or AppData mode)
3. **Web themes (`themes/`)** - HTML/CSS/JS frontends that connect via WebSocket to receive media updates

### Data Flow

```
Windows SMTC → smtc.dll → Go (smtc.go) → WebSocket → Web Browser
```

The C++ DLL uses a "dirty flag" pattern: `RetrieveDirtyData()` returns a bitmask indicating which data fields have changed since the last call, allowing efficient update batching.

### Build Tags

- Default (`main.go`): Native Windows GUI application with system tray
- `smtc_test` tag (`smtc_testmain.go`): Console test mode that polls SMTC and prints to stdout

## Build Commands

```batch
# Full build (builds C++ DLL and Go executable)
build.bat

# Manual build - C++ DLL only (requires Visual Studio 2022/2026)
cmake -B build -Hc -G "Visual Studio 18 2026"
cmake --build build --config MinSizeRel --target smtc_c

# Manual build - Go executable only
go build -ldflags="-s -w -H windowsgui" -o dist/SmtcNowPlaying.exe

# Test mode build
go build -tags smtc_test -o test.exe
```

**Important**: The Go executable expects `smtc.dll` to be present in the same directory or in the system PATH. The DLL is built to `dist/smtc.dll` by `build.bat`.

## Configuration

Two configuration modes:
- **Portable**: `portable_config.json` alongside executable
- **Installed**: `%APPDATA%/soarqin/smtc-now-playing/config.json`

Key settings:
- `port`: HTTP server port (default: 16321)
- `theme`: Selected theme directory name
- `autoStart`: Windows startup shortcut
- `startMinimized`: Launch to system tray only
- `previewWindow`: Show WebView2 preview
- `alwaysOnTop`: Preview window z-order

## WebSocket Protocol

Clients connect to `ws://localhost:<port>/ws`

Message format:
```json
{
  "type": "info" | "progress",
  "data": { ... }
}
```

**Info type**: `{ title, artist, albumArt: "/albumArt/<checksum>" }`
**Progress type**: `{ position, duration, status }`

Status values: Closed=0, Opened=1, Changing=2, Stopped=3, Playing=4, Paused=5

## Theme Development

Themes are located in `themes/<name>/`. Each theme must have:
- `index.html` - Main HTML file
- Optional CSS/JS files

Shared utilities available at `/script/functions.js` for WebSocket connection handling.

HTTP endpoints:
- `/<file>` - Serve files from selected theme directory
- `/script/<file>` - Serve files from `script/` directory
- `/albumArt/<checksum>` - Serve cached album art images
- `/ws` - WebSocket endpoint

## C++ Code Style

Follow `.editorconfig`:
- 4-space indentation
- K&R-style braces (opening brace on same line)
- No trailing whitespace

## Key Dependencies

**Go**:
- `github.com/ebitengine/purego` - FFI for calling C DLL without CGo
- `github.com/lxzan/gws` - WebSocket server
- `github.com/rodrigocfd/windigo` - Win32 GUI bindings
- `github.com/soarqin/go-webview2` - WebView2 wrapper

**C++**:
- Windows SDK WinRT C++/WinRT projections
- CMake 3.15+
- Visual Studio 2022/2026
