@echo off

mkdir dist >NUL 2>&1

go build -ldflags="-s -w -H windowsgui -X main.Version=1.2.0" -o dist/SmtcNowPlaying.exe

:: Create portable zip
echo Creating portable zip...
powershell -Command "Compress-Archive -Force -Path 'dist\SmtcNowPlaying.exe','themes','script','README.md' -DestinationPath 'dist\smtc-now-playing-v1.2.0-portable.zip'"
echo Done! Files in dist\:
dir dist\
