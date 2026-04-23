# smtc-now-playing

Display "Now Playing" information from Windows System Media Transport Controls (SMTC) as a web page. Works with any media player that reports to Windows SMTC: Spotify, browsers, Windows Media Player, and more.

## Features

- Real-time track info (title, artist, album art, album title)
- Playback status and progress with client-side interpolation
- WebSocket API for live updates
- REST API for polling and media control
- Customizable web themes (four built-in themes included)
- System tray integration with device selection
- Optional WebView2 preview window
- Hot-reload for theme development

## Quick Start

1. Download the latest release from [Releases](https://github.com/soarqin/smtc-now-playing/releases)
2. Extract and run `SmtcNowPlaying.exe`
3. Add `http://localhost:11451` as a browser source in OBS (or open it in any browser)

The page updates automatically whenever your media player changes tracks or playback state.

### Headless mode

Run with `--headless` to start the HTTP/WebSocket server without any GUI or system tray. Useful for running as a background service or in environments without a display.

```
SmtcNowPlaying.exe --headless
```

## Configuration

The app looks for config in two places, in order:

1. `portable_config.json` alongside the executable (portable mode)
2. `%APPDATA%\soarqin\smtc-now-playing\config.json` (installed mode)

If neither file exists, all defaults apply. Existing v1 flat config files are auto-migrated to the v2 nested format on first run.

### Example config

```json
{
  "server": {
    "port": 11451,
    "allowRemote": false,
    "hotReload": false
  },
  "ui": {
    "theme": "default",
    "autoStart": false,
    "startMinimized": false,
    "showPreviewWindow": false,
    "previewAlwaysOnTop": true
  },
  "smtc": {
    "selectedDevice": ""
  },
  "logging": {
    "level": "info",
    "debug": false
  }
}
```

### Config fields

**`server`**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | int | `11451` | HTTP server port |
| `allowRemote` | bool | `false` | Allow media control endpoints from non-localhost addresses |
| `hotReload` | bool | `false` | Watch theme files and reload connected clients on change |

**`ui`**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `theme` | string | `"default"` | Theme folder name inside `themes/` |
| `autoStart` | bool | `false` | Launch with Windows |
| `startMinimized` | bool | `false` | Start minimized to system tray |
| `showPreviewWindow` | bool | `false` | Show a WebView2 preview window |
| `previewAlwaysOnTop` | bool | `true` | Keep preview window on top of other windows |

**`smtc`**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `selectedDevice` | string | `""` | App ID of the SMTC session to monitor (empty = auto) |

**`logging`**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `"info"` | Log level: `"debug"`, `"info"`, `"warn"`, `"error"` |
| `debug` | bool | `false` | Enable verbose debug logging (shorthand for `level: "debug"`) |

## WebSocket API

Connect to `ws://localhost:11451/ws`. The server uses a v2 envelope format for all messages.

### Message envelope

Every message follows this structure:

```json
{
  "type": "<message-type>",
  "v": 2,
  "id": "abc123",
  "ts": 1711900000000,
  "data": { ... }
}
```

| Field | Description |
|-------|-------------|
| `type` | Message type string |
| `v` | Protocol version (always `2`) |
| `id` | Message ID (string); echoed in `ack` responses |
| `ts` | Server timestamp in milliseconds |
| `data` | Message payload (type-specific) |

### Server-to-client messages

#### `hello`

Sent immediately on connect. Carries server version and current state.

```json
{
  "type": "hello",
  "v": 2,
  "id": "...",
  "ts": 1711900000000,
  "data": {
    "version": "2.0.0",
    "info": { ... },
    "progress": { ... },
    "sessions": [ ... ]
  }
}
```

#### `info`

Sent when track metadata changes.

```json
{
  "type": "info",
  "v": 2,
  "id": "...",
  "ts": 1711900000000,
  "data": {
    "title": "Track Title",
    "artist": "Artist Name",
    "albumArt": "/albumArt/a3f2c1...",
    "albumTitle": "Album Name",
    "albumArtist": "Album Artist",
    "playbackType": 1,
    "sourceApp": "Spotify.exe"
  }
}
```

`albumArt` is a URL path to the current album art image served by the app. Fetch it with a normal `<img src="...">` tag. It's empty when no art is available.

`playbackType` values: `0` = Unknown, `1` = Music, `2` = Video, `3` = Image.

#### `progress`

Sent approximately every 200ms while media is active.

```json
{
  "type": "progress",
  "v": 2,
  "id": "...",
  "ts": 1711900000000,
  "data": {
    "position": 120,
    "duration": 240,
    "status": 4,
    "playbackRate": 1.0,
    "isShuffleActive": true,
    "autoRepeatMode": 0,
    "lastUpdatedTime": 1711900000000
  }
}
```

`position` and `duration` are in seconds. `lastUpdatedTime` is a Unix timestamp in milliseconds.

`isShuffleActive` can be `true`, `false`, or `null` (not supported by the current player).

`autoRepeatMode` values: `0` = None, `1` = Track, `2` = List.

##### Status values

| Value | Meaning |
|-------|---------|
| `0` | Closed |
| `1` | Opened |
| `2` | Changing |
| `3` | Stopped |
| `4` | Playing |
| `5` | Paused |

#### `sessions`

Sent when the list of available SMTC sessions changes.

```json
{
  "type": "sessions",
  "v": 2,
  "id": "...",
  "ts": 1711900000000,
  "data": [
    {"AppID": "Spotify.exe", "Name": "Spotify", "SourceAppId": "Spotify.exe"}
  ]
}
```

#### `reload`

Sent to all clients when hot-reload is enabled and a theme file changes.

```json
{"type": "reload", "v": 2, "id": "...", "ts": 1711900000000}
```

#### `pong`

Response to a client `ping`.

```json
{"type": "pong", "v": 2, "id": "...", "ts": 1711900000000}
```

#### `ack`

Response to a client `control` command.

```json
{
  "type": "ack",
  "v": 2,
  "id": "...",
  "ts": 1711900000000,
  "data": {
    "id": "<original-message-id>",
    "success": true,
    "error": ""
  }
}
```

### Client-to-server messages

#### `ping`

```json
{"type": "ping", "v": 2, "id": "client-001", "ts": 1711900000000}
```

#### `control`

Send a media control command. The server responds with an `ack`.

```json
{
  "type": "control",
  "v": 2,
  "id": "client-002",
  "ts": 1711900000000,
  "data": {
    "action": "play"
  }
}
```

Available actions: `play`, `pause`, `stop`, `toggle`, `next`, `previous`, `seek` (requires `position` in ms), `shuffle` (requires `active` bool), `repeat` (requires `mode` int).

## REST API

All endpoints are at `http://localhost:11451`.

### GET /api/now-playing

Returns the current track info and progress. Returns `404` when no active SMTC session exists.

```json
{
  "info": {
    "title": "Track Title",
    "artist": "Artist Name",
    "albumArt": "/albumArt/a3f2c1...",
    "albumTitle": "Album Name",
    "albumArtist": "Album Artist",
    "playbackType": 1,
    "sourceApp": "Spotify.exe"
  },
  "progress": {
    "position": 120,
    "duration": 240,
    "status": 4,
    "playbackRate": 1.0,
    "isShuffleActive": null,
    "autoRepeatMode": 0,
    "lastUpdatedTime": 1711900000000
  }
}
```

### GET /api/devices

### GET /api/sessions

Both return the same thing: a JSON array of available SMTC sessions.

```json
[
  {"AppID": "Spotify.exe", "Name": "Spotify", "SourceAppId": "Spotify.exe"}
]
```

### GET /api/capabilities

Returns which controls the current session supports.

```json
{
  "isPlayEnabled": true,
  "isPauseEnabled": true,
  "isStopEnabled": false,
  "isNextEnabled": true,
  "isPreviousEnabled": true,
  "isSeekEnabled": true,
  "isShuffleEnabled": true,
  "isRepeatEnabled": true
}
```

### Media control endpoints

All control endpoints use `POST`. By default they only accept requests from localhost. Set `server.allowRemote: true` in config to allow remote access.

All return `{"success": true}` on success or `{"success": false, "error": "..."}` on failure.

| Endpoint | Body | Description |
|----------|------|-------------|
| `POST /api/control/play` | none | Resume playback |
| `POST /api/control/pause` | none | Pause playback |
| `POST /api/control/stop` | none | Stop playback |
| `POST /api/control/toggle` | none | Toggle play/pause |
| `POST /api/control/next` | none | Skip to next track |
| `POST /api/control/previous` | none | Skip to previous track |
| `POST /api/control/seek` | `{"position": 12345}` | Seek to position in milliseconds |
| `POST /api/control/shuffle` | `{"active": true}` | Enable or disable shuffle |
| `POST /api/control/repeat` | `{"mode": 0}` | Set repeat mode (0=None, 1=Track, 2=List) |

## Theme Development

Themes live in the `themes/` directory. Each theme is a folder (e.g. `themes/default/`) containing at minimum an `index.html`. The built-in themes (`default`, `mini`, `new-horizontal`, `new-vertical`) are good references.

The shared `script/functions.js` handles WebSocket connection, reconnection, and message dispatch. Your theme's HTML just needs to define a few callback functions.

### Required callbacks

Your theme must define these on `window`:

```js
window.onLoaded = function () {
    // Called once after DOMContentLoaded. Set up your UI here.
}

window.setTrackInfo = function (title, artist) {
    // Called when the track title or artist changes.
}

window.setAlbumArt = function (albumArtUrl) {
    // Called with the URL path to the album art image, or empty string when none.
    // Use it as: img.src = albumArtUrl || 'placeholder.png'
}

window.setProgress = function (position, duration) {
    // Called with position and duration in seconds (floats during interpolation).
}

window.setPlayingStatus = function (status) {
    // Called with an integer status value (0-5, see table above).
}
```

### Optional callbacks

```js
window.setExtendedInfo = function ({albumTitle, albumArtist, playbackType, sourceApp}) {
    // Called alongside setTrackInfo with additional metadata.
}

window.setExtendedProgress = function ({playbackRate, isShuffleActive, autoRepeatMode, lastUpdatedTime}) {
    // Called alongside setProgress with additional playback state.
}
```

### CSS state classes

`functions.js` applies one of these classes to the root `<html>` element based on playback state:

| Class | When applied |
|-------|-------------|
| `playing` | Status is 4 (Playing) |
| `paused` | Status is 5 (Paused) |
| `stopped` | Status is 0-3 (not playing or paused) |
| `idle` | After `hideDelay` ms in stopped state |

Use these to show/hide or animate your overlay:

```css
html.idle .player-card {
    opacity: 0;
    transition: opacity 0.5s;
}

html.transitioning .track-info {
    opacity: 0;
}
```

The `transitioning` class is added briefly (300ms) when the track changes, so you can animate the old info out before the new info appears.

### URL parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `hideDelay` | `5000` | Milliseconds to wait in stopped state before applying the `idle` class |
| `maxWidth` | none | Max width in pixels for `.player-card` |
| `artWidth` | none | Width and height in pixels for `.album-art` |

Example: `http://localhost:11451?hideDelay=3000&maxWidth=400`

## Building from Source

See [docs/build.md](docs/build.md).

## Development

See [docs/development.md](docs/development.md).

## Requirements

- Windows 10 or Windows 11
- WebView2 runtime (pre-installed on Windows 11; available from Microsoft for Windows 10)

## License

MIT License
