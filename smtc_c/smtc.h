#pragma once

#include <windows.h>
#include <winrt/Windows.Media.Control.h>

#include <mutex>
#include <atomic>
#include <functional>
#include <string>
#include <vector>

using namespace winrt;
using namespace Windows::Media::Control;

class Smtc {
public:
    Smtc();
    ~Smtc();

    int init();
    void update();
    /*
     * returns int with bits:
     *  bit0: info dirty (artist, title, thumbnail)
     *  bit1: progress dirty (position, duration, status)
     * artist: 256 characters (UTF-16)
     * title: 256 characters (UTF-16)
     * thumbnailPath: 1024 characters (UTF-16)
     * status:
     *  0: Closed
     *  1: Opened
     *  2: Changing
     *  3: Stopped
     *  4: Playing
     *  5: Paused
     */
    int retrieveDirtyData(const wchar_t **artist, const wchar_t **title, const wchar_t **thumbnailContentType, const uint8_t **thumbnailData, int *thumbnailLength, int *position, int *duration, int *status);

private:
    void getMediaProperties();
    void checkUpdateOfThumbnail();
    void propertyChanged();

    GlobalSystemMediaTransportControlsSessionManager sessionManager_ = nullptr;
    GlobalSystemMediaTransportControlsSession currentSession_ = nullptr;
    GlobalSystemMediaTransportControlsSessionMediaProperties currentProperties_ = nullptr;

    std::wstring currentArtist_;
    std::wstring currentTitle_;
    std::wstring currentThumbnailContentType_;
    std::vector<uint8_t> currentThumbnailData_;
    int currentPosition_ = 0;
    int currentDuration_ = 0;
    GlobalSystemMediaTransportControlsSessionPlaybackStatus currentStatus_ = GlobalSystemMediaTransportControlsSessionPlaybackStatus::Closed;
    std::atomic<bool> mediaChanged_ = false;
    std::atomic<bool> mediaPropertyChanged_ = false;
    std::atomic<bool> infoDirty_ = false;
    std::atomic<bool> progressDirty_ = false;
};
