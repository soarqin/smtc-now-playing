# Development Guidelines

## Project Standards

### ⚠️ IMPORTANT: Language Requirements

**All code comments and documentation MUST be written in English.**

This ensures:
- Code maintainability across international teams
- Consistency throughout the codebase
- Better accessibility for AI agents and developers

## Go Code Style

### Naming Conventions

- **Packages**: Single word, lowercase (e.g., `config`, `smtc`, `server`)
- **Types**: PascalCase (e.g., `WebServer`, `infoDetail`)
- **Functions/Methods**: 
  - Exported: PascalCase
  - Unexported: camelCase
- **Interfaces**: PascalCase with `-er` suffix when appropriate
- **Constants**: PascalCase or UPPER_CASE for constant groups
- **Receiver names**: Short abbreviations (e.g., `me` for Gui, `srv` for WebServer, `ni` for NotifyIcon)

### Imports

Group imports by category with blank lines between:

```go
import (
    "fmt"
    "sync"

    "github.com/lxzan/gws"

    "smtc-now-playing/internal/config"
)
```

### Error Handling

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

- Return errors as the last return value
- Use `if err != nil` pattern
- For critical errors, use `panic` in init functions or `log.Fatalln`
- For GUI errors, show message box: `wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)`

### Struct Definitions

```go
type Config struct {
    Port               int    `json:"port"`
    Theme              string `json:"theme"`
    AutoStart          bool   `json:"autostart"`
}
```

- Group related fields together
- Align field types in columns when multiple fields
- Use struct tags for JSON serialization

### Comments

- Minimal comments; prefer self-documenting code
- Use `//` for single-line comments
- Comment exported types and functions only when necessary

### Build Tags

- Main GUI application: `//go:build !smtc_test`
- Test console application: `//go:build smtc_test`

### Concurrency

```go
srv.wsConnectionsMutex.Lock()
defer srv.wsConnectionsMutex.Unlock()
for conn := range srv.wsConnections {
    conn.WriteMessage(gws.OpcodeText, data)
}
```

- Use `sync.Mutex` with `defer mutex.Unlock()` pattern
- Use `sync.WaitGroup` for goroutine coordination

## C++ Code Style

Based on `.editorconfig`:

### Formatting

- **Indentation**: 4 spaces (no tabs)
- **Braces**: K&R style (opening brace on same line)
- **Pointer alignment**: Left-aligned (`int* ptr`, `const wchar_t** artist`)
- **Spaces**: 
  - Space after keywords in control flow
  - Space before block open brace
  - Space around binary operators
  - No space before comma, space after comma

### Naming Conventions

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

- **Classes**: PascalCase (e.g., `Smtc`)
- **Functions/Methods**: camelCase (e.g., `retrieveDirtyData`)
- **Member variables**: camelCase with trailing underscore (e.g., `currentArtist_`)
- **Private methods**: camelCase

### C++/WinRT Specifics

- Use `winrt` namespace for WinRT types
- Use `nullptr` for null pointers
- Use `std::wstring` for UTF-16 strings
- Use `std::atomic` for thread-safe flags

## Architecture

### Data Flow

```
Windows SMTC → smtc.dll → internal/smtc → internal/server → WebSocket → Web Browser
                              ↓
                        internal/gui → WebView2 (optional preview)
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

```cpp
// Returns bitmask indicating changed fields
// Bit 0: info dirty (artist, title, thumbnail)
// Bit 1: progress dirty (position, duration, status)
unsigned int RetrieveDirtyData();
```

### WebSocket Protocol

- Endpoint: `ws://localhost:<port>/ws`
- Message types: `info`, `progress`
- Format: `{"type": "<type>", "data": {...}}`

## Configuration

Two modes:

- **Portable**: `portable_config.json` alongside executable
- **Installed**: `%APPDATA%/soarqin/smtc-now-playing/config.json`

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
