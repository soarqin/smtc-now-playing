# smtc-now-playing

Display "Now Playing" information from Windows System Media Transport Controls (SMTC) as a web page.

## Features

- 🎵 Real-time track information (artist, title, album art)
- 🎧 Playback status and progress
- 🌐 WebSocket API for live updates
- 🎨 Customizable web themes
- 💻 System tray integration
- 🔔 Windows notifications

## Requirements

- Windows 10/11
- Visual Studio 2022/2026 with C++ workload (for building)
- Go 1.21+ (for building)
- WebView2 runtime (usually pre-installed on Windows 10/11)

## Installation

### Pre-built Binary

Download the latest release from the [Releases](https://github.com/soarqin/smtc-now-playing/releases) page.

### Build from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/soarqin/smtc-now-playing.git
   cd smtc-now-playing
   ```

2. Run the build script:
   ```batch
   build.bat
   ```

   Or build manually:
   ```batch
   # Build C++ DLL
   cmake -B build -Hc -G "Visual Studio 18 2026"
   cmake --build build --config MinSizeRel --target smtc_c

   # Build Go executable
   go build -ldflags="-s -w -H windowsgui" -o dist/SmtcNowPlaying.exe
   ```

## Usage

1. Run `SmtcNowPlaying.exe`
2. The app will start a local web server (default port: 8080)
3. Open your browser to `http://localhost:8080`
4. The page will automatically display track information from any media player using Windows SMTC

## Configuration

The application supports two configuration modes:

### Portable Mode

Create `portable_config.json` in the same directory as the executable:

```json
{
  "port": 8080,
  "theme": "default",
  "autostart": false,
  "show_notifications": true,
  "minimize_to_tray": true
}
```

### Installed Mode

Configuration is stored in `%APPDATA%/soarqin/smtc-now-playing/config.json`

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | int | 8080 | HTTP server port |
| `theme` | string | "default" | Web theme name |
| `autostart` | bool | false | Start with Windows |
| `show_notifications` | bool | true | Show track change notifications |
| `minimize_to_tray` | bool | true | Minimize to system tray |

## WebSocket API

Connect to `ws://localhost:<port>/ws` to receive real-time updates.

### Message Types

**info** - Track metadata:
```json
{
  "type": "info",
  "data": {
    "artist": "Artist Name",
    "title": "Track Title",
    "album": "Album Name",
    "thumbnail": "data:image/jpeg;base64,..."
  }
}
```

**progress** - Playback progress:
```json
{
  "type": "progress",
  "data": {
    "position": 120,
    "duration": 240,
    "status": "playing"
  }
}
```

### Status Values

- `playing` - Media is playing
- `paused` - Media is paused
- `stopped` - Media is stopped

## Development

### Project Structure

```
smtc-now-playing/
├── c/                  # C++ DLL source
│   └── smtc_c.cpp
├── internal/           # Go packages
│   ├── config/         # Configuration handling
│   ├── gui/            # Windows GUI and system tray
│   ├── server/         # HTTP/WebSocket server
│   ├── smtc/           # SMTC DLL interface
│   └── webview/        # WebView2 preview window
├── web/                # Web frontend
│   ├── index.html
│   ├── style.css
│   └── script.js
├── build.bat           # Build script
└── main.go             # Application entry point
```

### Test Mode

Build a console test application:

```batch
go build -tags smtc_test -o test.exe
.\test.exe
```

This will poll SMTC and print track information to the console.

## Architecture

```
Windows SMTC → smtc.dll → internal/smtc → internal/server → WebSocket → Web Browser
                              ↓
                        internal/gui → WebView2 (optional preview)
```

The C++ DLL uses a "dirty flag" pattern to efficiently communicate changes:
- Bit 0: Info changed (artist, title, thumbnail)
- Bit 1: Progress changed (position, duration, status)

## Technologies

- **Backend**: Go with purego FFI
- **SMTC Access**: C++/WinRT
- **GUI**: Win32 API via windigo
- **WebSocket**: gws library
- **Frontend**: HTML/CSS/JavaScript

## License

MIT License

## Author

soarqin
