# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - Unreleased

### Breaking Changes
- WebSocket protocol upgraded to v2: envelope format `{type, v:2, id, ts, data}`, hello handshake on connect, bidirectional control commands with ack, server heartbeat
- Config file format changed to nested structure; existing v1 flat configs are auto-migrated on first run
- SMTC event callback API replaced with channel-based `Subscribe()`/`Unsubscribe()` fan-out
- Binary layout changed: `cmd/smtc-now-playing/` and `cmd/smtc-test/` (was root `main.go` + `smtc_testmain.go`)
- `smtc_test` build tag removed; use `go build ./cmd/smtc-test` instead

### Added
- `--headless` flag for no-GUI server-only mode
- GitHub Actions CI workflow (build, test, lint, vulncheck on every PR)
- `go tool` directive pins `golangci-lint` and `govulncheck` in `go.mod`
- `internal/domain` package: shared data types (InfoData, ProgressData, SessionInfo, etc.)
- `internal/wsproto` package: WebSocket v2 protocol types and helpers
- `internal/version` package: app version exported for both binaries
- Per-subscriber drop counter for event fan-out
- Sentinel errors: `smtc.ErrNoSession`, `config.ErrInvalidConfig`, `server.ErrServerShutdown`

### Changed
- All packages: context.Context propagation for cancellable/blocking operations
- Error handling: all errors wrapped with `fmt.Errorf("%w", err)`, `errors.Is`/`errors.As` used throughout
- Logging: `log/slog` enriched with subsystem label per package
- HTTP server: timeouts set (ReadTimeout, WriteTimeout, IdleTimeout), graceful shutdown via `context.Background()`-derived timeout
- Server: hub actor pattern replaces 5 mutexes; zero sync.Mutex in server non-test code
- Server: Go 1.22 stdlib routing (`GET /path/{wildcard}` patterns)
- Config: constructor injection replaces global singleton (`config.Get()` removed)
- SMTC goroutine: `Run(ctx)` replaces `Start()`/`Stop()` lifecycle

### Removed
- `config.Get()` global singleton
- SMTC 4-callback `Options` struct (OnInfo, OnProgress, OnSessionsChanged, OnSelectedDeviceChange)
- `smtc_testmain.go` at root (replaced by `cmd/smtc-test/`)
- Root `main.go` and `version.go` (moved to `cmd/smtc-now-playing/`)

### Fixed
- Silent error swallowing in WinRT async operations (now wrapped with `fmt.Errorf`)
- Previously dropped SMTC events now logged with per-subscriber drop counter

### Migration Notes
- WebSocket clients must update to v2 protocol (envelope format changed)
- Config files: existing flat JSON is auto-migrated on first launch; no manual edit needed
- Themes: HTML/CSS unchanged; `functions.js` updated automatically with the binary
- Installer: same binary name (`SmtcNowPlaying.exe`), same AppId GUID

## [Unreleased]

### Fixed
- Cover art occasionally disappearing right after a song switch: when a follow-up `info` event delivered the album art within the 300 ms track-change transition, the scheduled `setTimeout` in `script/functions.js` would fire later with its closure-captured (often empty) album-art URL and overwrite the freshly-applied one. Pending track info is now held in a shared variable that every follow-up event updates in place, so the timeout always applies the latest values.
- Playback progress failing to reset to the start when replaying the same song: `handlePlaybackInfoChanged` fired `OnProgress` with only `Status` set, zeroing the client's `position`/`duration`/`lastUpdatedTime`/`playbackRate`. If the next 200 ms `readTimelineAndProgress` tick then dedup-matched (e.g. `currentPosition` was already 0), no fresh data was sent and the UI stayed stuck at `0:00/`. The handler now delegates to `readTimelineAndProgress`, guaranteeing a complete progress snapshot on every status transition.

## [1.2.2] - 2026-04-22

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
