# Build Instructions

## Requirements

- Windows 10/11
- Go 1.22 or later

## Building

### Prerequisites
- Go 1.22 or later
- Windows 10/11

### Build
```batch
build.bat
```
This builds the Go executable to `dist/SmtcNowPlaying.exe`.

### Manual build
```batch
go build -ldflags="-s -w -H windowsgui" -o dist/SmtcNowPlaying.exe
```

## Test Mode Build

Build a console application for testing:

```batch
go build -tags smtc_test -o test.exe
.\test.exe
```

This will poll SMTC and print track information to the console.

## Lint and Format

```batch
go fmt ./...
go vet ./...
```
