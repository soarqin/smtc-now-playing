@echo off

mkdir dist >NUL 2>&1

go build -ldflags="-s -w -H windowsgui" -o dist/SmtcNowPlaying.exe
