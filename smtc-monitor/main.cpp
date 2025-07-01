#include "smtc.h"

#include <windows.h>

#include <cstdio>
#include <fcntl.h>
#include <io.h>

int wmain(int argc, wchar_t** argv) {
    SetConsoleCP(CP_UTF8);
    SetConsoleOutputCP(CP_UTF8);
    // set stdio encoding to utf-8
    _setmode(_fileno(stdout), _O_U8TEXT);
    _setmode(_fileno(stderr), _O_U8TEXT);
    _setmode(_fileno(stdin), _O_U8TEXT);
    Smtc smtc;
    smtc.onInfoUpdate([&smtc](const std::wstring& artist, const std::wstring& title, const std::wstring& thumbnailPath) {
        fwprintf(stdout, L"I\t%ls\t%ls\t%ls\n", artist.c_str(), title.c_str(), thumbnailPath.c_str());
        fflush(stdout);
    });
    smtc.onProgressUpdate([](int currentTime, int duration, GlobalSystemMediaTransportControlsSessionPlaybackStatus status) {
        fwprintf(stdout, L"P\t%d\t%d\t%d\n", currentTime, duration, (int)status);
        fflush(stdout);
    });
    smtc.init();
    while (true) {
        std::this_thread::sleep_for(std::chrono::milliseconds(200));
        smtc.update();
    }
    return 0;
}
