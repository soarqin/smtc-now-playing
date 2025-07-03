#pragma once

#include <windows.h>
#include <winrt/Windows.Media.Control.h>

#include <mutex>
#include <atomic>
#include <functional>
#include <string>

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
    int retrieveDirtyData(wchar_t *artist, wchar_t *title, wchar_t *thumbnailPath, int *position, int *duration, int *status);

    inline const std::wstring &getArtist() const { return currentArtist_; }
    inline const std::wstring &getTitle() const { return currentTitle_; }
    inline const std::wstring &getThumbnailPath() const { return currentThumbnailPath_; }
    inline int getPosition() const { return currentPosition_; }
    inline int getDuration() const { return currentDuration_; }
    inline GlobalSystemMediaTransportControlsSessionPlaybackStatus getStatus() const { return currentStatus_; }

private:
    void getMediaProperties();
    void checkUpdateOfThumbnail();
    void propertyChanged();

    GlobalSystemMediaTransportControlsSessionManager sessionManager_ = nullptr;
    GlobalSystemMediaTransportControlsSession currentSession_ = nullptr;
    GlobalSystemMediaTransportControlsSessionMediaProperties currentProperties_ = nullptr;
    std::mutex sessionMutex_;

    std::wstring currentArtist_;
    std::wstring currentTitle_;
    std::wstring currentThumbnailPath_;
    int currentThumbnailLength_ = 0;
    int currentPosition_ = 0;
    int currentDuration_ = 0;
    GlobalSystemMediaTransportControlsSessionPlaybackStatus currentStatus_ = GlobalSystemMediaTransportControlsSessionPlaybackStatus::Closed;
    std::atomic<bool> mediaChanged_ = false;
    std::atomic<bool> mediaPropertyChanged_ = false;
    std::atomic<bool> infoDirty_ = false;
    std::atomic<bool> progressDirty_ = false;
};
