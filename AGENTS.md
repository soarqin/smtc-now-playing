# AGENTS.md

Guidelines for agentic coding agents working in this repository.

For detailed development guidelines, see [docs/development.md](docs/development.md).

## Project Overview

**smtc-now-playing** is a Windows desktop application that displays "Now Playing" information from Windows System Media Transport Controls (SMTC) as a web page. It uses pure Go with winrt-go for WinRT integration and a Go application for the HTTP/WebSocket server and native GUI.

## Project Standards

### ⚠️ IMPORTANT: Language Requirements

**All code comments and documentation MUST be written in English.**

See [docs/development.md](docs/development.md) for full coding standards.

## Build Commands

### Full Build
```batch
build.bat
```
This builds the Go executable.

### Manual Build - Main Binary
```batch
go build -ldflags="-s -w -H windowsgui -X smtc-now-playing/internal/version.Version=<version>" -o dist/SmtcNowPlaying.exe ./cmd/smtc-now-playing
```

### Dev Test Tool (console, not shipped in releases)
```batch
go build -o dist/smtc-test.exe ./cmd/smtc-test
```
Builds a console application that listens for SMTC events and prints to stdout.

### Lint/Format
```batch
go fmt ./...
go vet ./...
```

## Testing

Test coverage exists for config, domain, wsproto, smtc (control and helpers), server, and updater packages. Run tests with:

```batch
go test ./...
```

## Code Style Guidelines

See [docs/development.md](docs/development.md) for detailed code style guidelines including:
- Go naming conventions and error handling
- Struct definitions and concurrency patterns

## Architecture Notes

See [docs/development.md](docs/development.md) for detailed architecture documentation.

### Data Flow
```
Windows SMTC → winrt-go (WinRT COM) → internal/smtc → internal/server → WebSocket → Web Browser
                                           ↓
                                     internal/gui → WebView2 (optional preview)
```

### Project Structure

```
smtc-now-playing/
├── cmd/
│   ├── smtc-now-playing/   # Main application entry point
│   └── smtc-test/          # Dev console tool for SMTC event inspection
├── internal/               # Go packages
│   ├── config/             # Configuration handling
│   ├── domain/             # Shared data types (InfoData, ProgressData, SessionInfo, etc.)
│   ├── gui/                # Windows GUI and system tray
│   ├── server/             # HTTP/WebSocket server
│   ├── smtc/               # SMTC WinRT interface
│   ├── version/            # App version exported for both binaries
│   ├── webview/            # WebView2 preview window
│   └── wsproto/            # WebSocket v2 protocol types and helpers
├── themes/                 # Web themes
├── build.bat               # Build script
└── installer/              # Inno Setup installer script
```

### Package Dependencies

```
cmd/smtc-now-playing → internal/gui → internal/config
                                       internal/server → internal/smtc
                                                         internal/domain
                                                         internal/wsproto
                                       internal/webview
                       internal/version
```

No circular dependencies. Each internal package has a single responsibility.

### SMTC Interface
The Go smtc package uses a channel-based fan-out architecture:
- `Subscribe()` / `Unsubscribe()`: register/deregister event channels
- `Run(ctx)`: starts the SMTC goroutine; blocks until context is cancelled

### Configuration
Two modes:
- **Portable**: `portable_config.json` alongside executable
- **Installed**: `%APPDATA%/soarqin/smtc-now-playing/config.json`

Config is nested (v2 format). Existing v1 flat configs are auto-migrated on first run.

### WebSocket Protocol
- Endpoint: `ws://localhost:<port>/ws`
- Protocol version: v2
- Envelope format: `{"type": "<type>", "v": 2, "id": "<id>", "ts": <ms>, "data": {...}}`
- Message types (server→client): `hello`, `info`, `progress`, `sessions`, `reload`, `pong`, `ack`
- Message types (client→server): `ping`, `control`

## Dependencies

### Go
- `github.com/saltosystems/winrt-go` - WinRT COM bindings for SMTC
- `github.com/go-ole/go-ole` - WinRT COM foundation (RoInitialize, IUnknown)
- `github.com/lxzan/gws` - WebSocket server
- `github.com/rodrigocfd/windigo` - Win32 GUI bindings
- `github.com/soarqin/go-webview2` - WebView2 wrapper
- `golang.org/x/sys` - Windows syscall support
