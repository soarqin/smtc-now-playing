@echo off

mkdir dist >NUL 2>&1

go build -ldflags="-s -w -H windowsgui -X main.Version=1.2.0" -o dist/SmtcNowPlaying.exe
