#pragma once

#include <windows.h>
#include <winrt/Windows.Media.Control.h>

#include <mutex>
#include <functional>
#include <string>

using namespace winrt;
using namespace Windows::Media::Control;

class Smtc {
public:
    Smtc();
    ~Smtc();

    void start();
    void stop();

    inline void onInfoUpdate(std::function<void(const std::wstring&, const std::wstring&, const std::wstring&)> callback) {
        infoUpdateCallback_ = std::move(callback);
    }

    inline void onProgressUpdate(std::function<void(int currentTime, int duration, GlobalSystemMediaTransportControlsSessionPlaybackStatus)> callback) {
        progressUpdateCallback_ = std::move(callback);
    }

    inline const std::wstring &getArtist() const { return currentArtist_; }
    inline const std::wstring &getTitle() const { return currentTitle_; }
    inline const std::wstring &getThumbnailPath() const { return currentThumbnailPath_; }
    inline int getPosition() const { return currentPosition_; }
    inline int getDuration() const { return currentDuration_; }
    inline GlobalSystemMediaTransportControlsSessionPlaybackStatus getStatus() const { return currentStatus_; }

private:
    void getMediaProperties();
    void updateMediaProperties();
    void checkUpdateOfThumbnail();

    std::function<void(const std::wstring&, const std::wstring&, const std::wstring&)> infoUpdateCallback_;
    std::function<void(int currentTime, int duration, GlobalSystemMediaTransportControlsSessionPlaybackStatus)> progressUpdateCallback_;
    bool isRunning_ = false;
    GlobalSystemMediaTransportControlsSessionManager sessionManager_ = nullptr;
    GlobalSystemMediaTransportControlsSession currentSession_ = nullptr;
    GlobalSystemMediaTransportControlsSessionMediaProperties currentProperties_ = nullptr;
    std::recursive_mutex sessionMutex_;

    std::wstring currentArtist_;
    std::wstring currentTitle_;
    std::wstring currentThumbnailPath_;
    int currentThumbnailLength_ = 0;
    int currentPosition_ = 0;
    int currentDuration_ = 0;
    GlobalSystemMediaTransportControlsSessionPlaybackStatus currentStatus_ = GlobalSystemMediaTransportControlsSessionPlaybackStatus::Closed;
    bool mediaChanged_ = false;
    bool mediaPropertyChanged_ = false;
    bool infoDirty_ = false;
};
