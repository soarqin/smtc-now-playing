# AGENTS.md

Guidelines for agentic coding agents working in this repository.

## Project Overview

**smtc-now-playing** is a Windows desktop application that displays "Now Playing" information from Windows System Media Transport Controls (SMTC) as a web page. It uses a C++ DLL for WinRT integration and a Go application for the HTTP/WebSocket server and native GUI.

## Project Standards

### ⚠️ IMPORTANT: Language Requirements

**All code comments and documentation MUST be written in English.**

This is a strict project requirement to ensure:
- Code maintainability across international teams
- Consistency throughout the codebase
- Better accessibility for AI agents and developers

#### Naming Conventions
- **Packages**: Single word, lowercase
- **Types**: PascalCase (e.g., `WebServer`, `infoDetail`)
- **Functions/Methods**: PascalCase for exported, camelCase for unexported
- **Receiver names**: Short abbreviations (e.g., `me` for Gui, `srv` for WebServer, `ni` for NotifyIcon)

#### Imports
Group imports by category with blank lines between:
1. Standard library
2. External packages
3. Internal packages

```go
import (
    "fmt"
    "sync"

    "github.com/lxzan/gws"

    "smtc-now-playing/internal/config"
)
```

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

### Go Code Style

#### Naming Conventions
- **Packages**: Single word, lowercase (e.g., `config`, `smtc`, `server`)
- **Types**: PascalCase (e.g., `WebServer`, `infoDetail`)
- **Functions/Methods**: PascalCase for exported, camelCase for unexported
- **Interfaces**: PascalCase with `-er` suffix when appropriate
- **Constants**: PascalCase or UPPER_CASE for constant groups
- **Receiver names**: Short abbreviations (e.g., `me` for Gui, `srv` for WebServer, `ni` for NotifyIcon)

#### Error Handling
- Return errors as the last return value
- Use `if err != nil` pattern
- For critical errors, use `panic` in init functions or `log.Fatalln`
- For GUI errors, show message box: `wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)`

```go
func loadConfigFromFile(path string, cfg *Config) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()
    // ...
}
```

#### Struct Definitions
- Group related fields together
- Align field types in columns when multiple fields
- Use struct tags for JSON serialization

```go
type Config struct {
    Port               int    `json:"port"`
    Theme              string `json:"theme"`
    AutoStart          bool   `json:"autostart"`
}
```

#### Comments
- Minimal comments; prefer self-documenting code
- Use `//` for single-line comments
- Comment exported types and functions only when necessary

#### Build Tags
- Main GUI application: `//go:build !smtc_test`
- Test console application: `//go:build smtc_test`

#### Concurrency
- Use `sync.Mutex` with `defer mutex.Unlock()` pattern
- Use `sync.WaitGroup` for goroutine coordination

```go
srv.wsConnectionsMutex.Lock()
defer srv.wsConnectionsMutex.Unlock()
for conn := range srv.wsConnections {
    conn.WriteMessage(gws.OpcodeText, data)
}
```

### C++ Code Style

Based on `.editorconfig`:

- **Indentation**: 4 spaces (no tabs)
- **Braces**: K&R style (opening brace on same line)
- **Pointer alignment**: Left-aligned (`int* ptr`, `const wchar_t** artist`)
- **Spaces**: 
  - Space after keywords in control flow
  - Space before block open brace
  - Space around binary operators
  - No space before comma, space after comma

```cpp
class Smtc {
public:
    Smtc();
    ~Smtc();
    int init();
    void update();
private:
    std::wstring currentArtist_;
    int currentPosition_ = 0;
};
```

#### Naming Conventions
- **Classes**: PascalCase (e.g., `Smtc`)
- **Functions/Methods**: camelCase (e.g., `retrieveDirtyData`)
- **Member variables**: camelCase with trailing underscore (e.g., `currentArtist_`)
- **Private methods**: camelCase

#### C++/WinRT Specifics
- Use `winrt` namespace for WinRT types
- Use `nullptr` for null pointers
- Use `std::wstring` for UTF-16 strings
- Use `std::atomic` for thread-safe flags

## Architecture Notes

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