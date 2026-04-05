# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
