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
    smtc.init();
    while (true) {
        std::this_thread::sleep_for(std::chrono::milliseconds(200));
        smtc.update();
        wchar_t artist[256];
        wchar_t title[256];
        wchar_t thumbnailPath[1024];
        int currentTime;
        int duration;
        int status;
        auto dirty = smtc.retrieveDirtyData(artist, title, thumbnailPath, &currentTime, &duration, &status);
        if (dirty & 1) {
            fwprintf(stdout, L"I\t%ls\t%ls\t%ls\n", artist, title, thumbnailPath);
            fflush(stdout);
        }
        if (dirty & 2) {
            fwprintf(stdout, L"P\t%d\t%d\t%d\n", currentTime, duration, status);
            fflush(stdout);
        }
    }
    return 0;
}
