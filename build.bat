@echo off
setlocal

:: Extract version from version.go (the const Version = "x.y.z" line) so we
:: don't duplicate the version number across the codebase.
for /f "tokens=4 delims= " %%v in ('findstr /R /C:"^const Version" version.go') do (
	set VERSION=%%v
)
:: Strip surrounding quotes.
set VERSION=%VERSION:"=%

if "%VERSION%"=="" (
	echo Failed to parse version from version.go
	exit /b 1
)

echo Building smtc-now-playing v%VERSION%...

mkdir dist >NUL 2>&1

go build -ldflags="-s -w -H windowsgui -X main.Version=%VERSION%" -o dist/SmtcNowPlaying.exe
if errorlevel 1 exit /b 1

:: Create portable zip
echo Creating portable zip...
powershell -Command "Compress-Archive -Force -Path 'dist\SmtcNowPlaying.exe','themes','script','README.md' -DestinationPath 'dist\smtc-now-playing-v%VERSION%-portable.zip'"
echo Done! Files in dist\:
dir dist\

endlocal
