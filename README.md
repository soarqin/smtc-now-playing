# smtc-now-playing

Display "Now Playing" information from Windows System Media Transport Controls (SMTC) as a web page.

## Features

- 🎵 Real-time track information (artist, title, album art)
- 🎧 Playback status and progress
- 🌐 WebSocket API for live updates
- 🎨 Customizable web themes
- 💻 System tray integration
- 🔔 Windows notifications

## Quick Start

### Download

Get the latest release from [Releases](https://github.com/soarqin/smtc-now-playing/releases).

### Run

1. Extract the downloaded archive
2. Run `SmtcNowPlaying.exe`
3. Open your browser to `http://localhost:8080`

The page will automatically display track information from any media player using Windows SMTC.

## Configuration

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

### Message Format

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

## Building from Source

See [Build Instructions](docs/build.md).

## Development

See [Development Guidelines](docs/development.md).

## Requirements

- Windows 10/11
- WebView2 runtime (usually pre-installed)

## License

MIT License
