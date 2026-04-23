@echo off
setlocal

:: Extract version from internal/version package (the const Version = "x.y.z" line)
:: so we don't duplicate the version number across the codebase.
for /f "tokens=4 delims= " %%v in ('findstr /R /C:"^const Version" internal\version\version.go') do (
	set VERSION=%%v
)
:: Strip surrounding quotes.
set VERSION=%VERSION:"=%

if "%VERSION%"=="" (
	echo Failed to parse version from internal/version/version.go
	exit /b 1
)

echo Building smtc-now-playing v%VERSION%...

mkdir dist >NUL 2>&1

go build -ldflags="-s -w -H windowsgui -X smtc-now-playing/internal/version.Version=%VERSION%" -o dist/SmtcNowPlaying.exe ./cmd/smtc-now-playing
if errorlevel 1 exit /b 1

:: Uncomment to build the dev test tool (console binary, not shipped in releases):
:: go build -o dist/smtc-test.exe ./cmd/smtc-test

:: Create portable zip
echo Creating portable zip...
powershell -Command "Compress-Archive -Force -Path 'dist\SmtcNowPlaying.exe','themes','script','README.md' -DestinationPath 'dist\smtc-now-playing-v%VERSION%-portable.zip'"
echo Done! Files in dist\:
dir dist\

endlocal
