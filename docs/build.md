# Build Instructions

## Requirements

- Windows 10/11
- Visual Studio 2022/2026 with C++ workload
- Go 1.21+
- CMake 3.15+

## Quick Build

Use the build script to build everything:

```batch
build.bat
```

This builds both the C++ DLL and Go executable. Output will be in the `dist/` directory.

## Manual Build

### Step 1: Build C++ DLL

```batch
cmake -B build -Hc -G "Visual Studio 18 2026"
cmake --build build --config MinSizeRel --target smtc_c
```

The DLL will be built to `build/MinSizeRel/smtc.dll`.

### Step 2: Build Go Executable

```batch
go build -ldflags="-s -w -H windowsgui" -o dist/SmtcNowPlaying.exe
```

Copy `smtc.dll` to the same directory as the executable, or add it to your system PATH.

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

## Build Output

- `dist/SmtcNowPlaying.exe` - Main application
- `dist/smtc.dll` - SMTC interface DLL (must be alongside the exe or in PATH)
