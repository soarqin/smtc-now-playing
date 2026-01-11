@echo off

mkdir dist >NUL 2>&1
mkdir build >NUL 2>&1

cmake -B build -H. -G "Visual Studio 18 2026"
cmake --build build --config MinSizeRel --target smtc_c

copy /y build\bin\smtc.dll dist\smtc.dll >NUL 2>&1

pushd smtc-now-playing >NUL
go build -ldflags="-s -w -H windowsgui" -o ../dist/SmtcNowPlaying.exe
popd >NUL
