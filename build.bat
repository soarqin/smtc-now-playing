@echo off

mkdir dist >NUL 2>&1
mkdir build >NUL 2>&1

cmake -B build -H. -G "Visual Studio 17 2022"
cmake --build build --config MinSizeRel --target SmtcMonitor

copy /y build\bin\SmtcMonitor.exe .\dist\SmtcMonitor.mod

pushd smtc-now-playing >NUL
go build -ldflags="-s -w -H windowsgui" -o ../dist/SmtcNowPlaying.exe
popd >NUL
