# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Structured logging via log/slog (--debug flag)
- Full Go test infrastructure
- Graceful error handling (no more crashes on init failure)
- Regression tests for directory traversal, IPv6 loopback detection, and atomic config writes

### Security
- Fixed directory-traversal vulnerabilities in `/` (theme) and `/script/` handlers — arbitrary files outside the theme / script directories could previously be read over HTTP
- Tightened localhost detection for media-control endpoints: `isLocalhost` now uses `net.IP.IsLoopback()` so the whole 127.0.0.0/8 range and zone-qualified IPv6 loopback addresses are recognised

### Fixed
- Config file is now written atomically (temp file + rename). Killing the process mid-save no longer corrupts `config.json`
- `config.Save()` / `config.Load()` are now serialised via a mutex and surface directory-creation errors instead of silently dropping them
- `CreateMutex` single-instance check now distinguishes `ERROR_ALREADY_EXISTS` from real failures (invalid name, etc.) and closes the duplicate handle it receives so the kernel refcount stays accurate
- `main.go` cleans up the single-instance mutex *before* calling `os.Exit`, so the next launch always finds a released mutex
- `gui.go` no longer panics when `RegisterWindowMessage` fails; it logs and continues with tray auto-restore disabled
- System-tray popup-menu leak: the `Device` submenu is now destroyed when its label conversion fails, preventing an HMENU leak on every right-click in that error path
- WebSocket broadcast no longer holds the connections mutex while writing, so a single slow client can no longer block all other broadcasts or the `OnClose` callback
- `ReadLoop` goroutines now recover from panics and remove themselves from the connection set so a malformed frame can't orphan connections or crash the process
- Hot-reload file watcher now has an explicit shutdown channel and joins its goroutine on `Stop()`, eliminating a shutdown race where a pending debounce tick could fire against torn-down state
- `smtc.Stop()` waits (bounded) for the dedicated WinRT goroutine to finish unsubscribing event tokens, and guards `thumbnailRetryTimer` access with a mutex so concurrent `Stop()` / property-change races are safe
- `smtc.waitForAsync` no longer blocks indefinitely when `SetCompleted` fails or the WinRT async operation stalls — it logs and returns `AsyncStatusError` after a 5s timeout
- Thumbnail reader releases the outer `IRandomAccessStreamWithContentType` on every error path (was previously leaked on QueryInterface / size failures) and guards `getContentType` with `defer` so a panic can't leak the content-type interface
- `/api/now-playing` 404 responses are now JSON (`{"success":false,"error":"no active session"}`) to match the rest of the API
- `script/functions.js`: guarded `window.onLoaded` against undefined, wrapped the onmessage handler in `try/catch`, validated `JSON.parse` with a graceful fallback, and switched reconnect from a flat 1 s to 1-30 s exponential backoff

### Changed
- Unified the application version in a single constant (`version.go`); `build.bat` now reads it from there instead of duplicating it in two places

## [1.2.1] - 2026-04-12

### Fixed
- System tray icon not restored after Explorer crash/restart due to TaskbarCreated message handler registered with zero value and incomplete icon property restoration

## [1.2.0] - 2026-04-05

### Added
- New theme `modern`: dark minimal horizontal card with album art, progress bar, status indicator, entry/song-change animations
- New theme `stream-bar`: wide low-height OBS bottom bar overlay with full-width background progress, no album art
- New theme `minimal`: ultra-minimal two-column layout with title and artist separated by a full-height vertical line, fully transparent background, text shadow for light/dark background compatibility

## [1.1.0] - 2026-03-21

### Added
- Multi-device support: enumerate all SMTC sessions and select a specific media source
- Device selection UI: ComboBox in settings and submenu in system tray
- Device list and selection bridge methods in the HTTP/WebSocket server
- Pure Go SMTC reader using winrt-go and go-ole (replaces C++ DLL)

### Fixed
- Album art not displaying due to deduplication logic returning empty data
- Wrong IIDs for `IRandomAccessStream` and `IContentTypeProvider` causing WinRT errors
- Data races in SMTC event handlers and task references
- Album art deduplication and event handler race conditions

### Changed
- Server updated to event-driven SMTC API
- Test mode updated for new SMTC API
- Release workflow updated for pure Go build

### Removed
- C++ DLL and legacy CMake build infrastructure
- `purego` dependency (replaced by winrt-go)

## [1.0.0] - 2026-03-19

### Added
- Real-time track information (artist, title, album art)
- Playback status and progress tracking
- WebSocket API for live updates
- Customizable web themes
- System tray integration
- Windows notifications
- WebView2 preview window
- Portable and installed configuration modes
- Windows System Media Transport Controls (SMTC) integration
