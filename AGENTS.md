# AGENTS.md

Guidelines for agentic coding agents working in this repository.

For detailed development guidelines, see [docs/development.md](docs/development.md).

## Project Overview

**smtc-now-playing** is a Windows desktop application that displays "Now Playing" information from Windows System Media Transport Controls (SMTC) as a web page. It uses a C++ DLL for WinRT integration and a Go application for the HTTP/WebSocket server and native GUI.

## Project Standards

### ⚠️ IMPORTANT: Language Requirements

**All code comments and documentation MUST be written in English.**

See [docs/development.md](docs/development.md) for full coding standards.

## Build Commands

### Full Build
```batch
build.bat
```
This builds both the C++ DLL (via CMake) and the Go executable.

### Manual Build - C++ DLL Only
```batch
cmake -B build -Hc -G "Visual Studio 18 2026"
cmake --build build --config MinSizeRel --target smtc_c
```
Requires Visual Studio 2022/2026 with C++/WinRT support.

### Manual Build - Go Executable Only
```batch
go build -ldflags="-s -w -H windowsgui" -o dist/SmtcNowPlaying.exe
```
The Go executable requires `smtc.dll` in the same directory or system PATH.

### Test Mode Build
```batch
go build -tags smtc_test -o test.exe
```
Builds a console test application that polls SMTC and prints to stdout.

### Lint/Format
```batch
go fmt ./...
go vet ./...
```

## Testing

No unit tests currently exist in this codebase. Testing is done via the test mode build (`-tags smtc_test`) which provides console output for manual verification.

## Code Style Guidelines

See [docs/development.md](docs/development.md) for detailed code style guidelines including:
- Go naming conventions and error handling
- C++ formatting and C++/WinRT specifics
- Struct definitions and concurrency patterns

## Architecture Notes

See [docs/development.md](docs/development.md) for detailed architecture documentation.

### Data Flow
```
Windows SMTC → smtc.dll → internal/smtc → internal/server → WebSocket → Web Browser
                              ↓
                        internal/gui → WebView2 (optional preview)
```

### Project Structure

```
smtc-now-playing/
├── c/                  # C++ DLL source
├── internal/           # Go packages
│   ├── config/         # Configuration handling
│   ├── gui/            # Windows GUI and system tray
│   ├── server/         # HTTP/WebSocket server
│   ├── smtc/           # SMTC DLL interface
│   └── webview/        # WebView2 preview window
├── themes/             # Web themes
├── build.bat           # Build script
└── main.go             # Application entry point
```

### Package Dependencies

```
main.go → internal/gui → internal/config
                         internal/server → internal/smtc
                         internal/webview
```

No circular dependencies. Each internal package has a single responsibility.

### DLL Interface
The C++ DLL uses a "dirty flag" pattern:
- `RetrieveDirtyData()` returns a bitmask indicating changed fields
- Bit 0: info dirty (artist, title, thumbnail)
- Bit 1: progress dirty (position, duration, status)

### Configuration
Two modes:
- **Portable**: `portable_config.json` alongside executable
- **Installed**: `%APPDATA%/soarqin/smtc-now-playing/config.json`

### WebSocket Protocol
- Endpoint: `ws://localhost:<port>/ws`
- Message types: `info`, `progress`
- Format: `{"type": "<type>", "data": {...}}`

## Dependencies

### Go
- `github.com/ebitengine/purego` - FFI for calling C DLL without CGo
- `github.com/lxzan/gws` - WebSocket server
- `github.com/rodrigocfd/windigo` - Win32 GUI bindings
- `github.com/soarqin/go-webview2` - WebView2 wrapper
- `golang.org/x/sys` - Windows syscall support

### C++
- Windows SDK
- C++/WinRT (via NuGet in CMakeLists.txt)