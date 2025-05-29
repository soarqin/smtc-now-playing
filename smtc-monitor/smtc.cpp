#include "smtc.h"

#include <winrt/Windows.Storage.Streams.h>
#include <winrt/Windows.Foundation.h>
#include <windows.h>

#include <chrono>
#include <tuple>
#include <mutex>

using namespace Windows::Foundation;
using namespace Windows::Storage::Streams;

Smtc::Smtc() {
    init_apartment();
}

Smtc::~Smtc() {
    uninit_apartment();
}

template<typename T>
inline static std::tuple<AsyncStatus, T> WaitForAsyncOperation(IAsyncOperation<T> operation) {
    AsyncStatus status;
    while ((status = operation.Status()) == AsyncStatus::Started) {
        std::this_thread::sleep_for(std::chrono::milliseconds(20));
    }
    if (status == AsyncStatus::Completed) {
        return std::make_tuple(status, operation.GetResults());
    }
    return std::make_tuple(status, T(0));
}

template<typename T>
inline static AsyncStatus WaitForAsyncOperationNoReturn(IAsyncOperation<T> operation) {
    AsyncStatus status;
    while ((status = operation.Status()) == AsyncStatus::Started) {
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
    return status;
}

void Smtc::start() {
    isRunning_ = true;
    auto [status, sessionManager_] = WaitForAsyncOperation(GlobalSystemMediaTransportControlsSessionManager::RequestAsync());
    if (status != AsyncStatus::Completed) {
        return;
    }
    currentSession_ = sessionManager_.GetCurrentSession();
    auto propertyChanged = [this]() {
        currentSession_.MediaPropertiesChanged([&](const GlobalSystemMediaTransportControlsSession& sender, const MediaPropertiesChangedEventArgs& args) {
            std::lock_guard lock(sessionMutex_);
            mediaPropertyChanged_ = true;
        });
    };
    if (currentSession_) propertyChanged();
    sessionManager_.CurrentSessionChanged([&](const GlobalSystemMediaTransportControlsSessionManager& sender, const CurrentSessionChangedEventArgs& args) {
        std::lock_guard lock(sessionMutex_);
        mediaChanged_ = true;
        mediaPropertyChanged_ = true;
    });
    while (isRunning_) {
        std::this_thread::sleep_for(std::chrono::milliseconds(500));
        bool progressDirty = false;
        {
            std::lock_guard lock(sessionMutex_);
            if (mediaChanged_) {
                mediaChanged_ = false;
                currentProperties_ = nullptr;
                auto oldSession = currentSession_;
                currentSession_ = sessionManager_.GetCurrentSession();
                if (!currentSession_) {
                    mediaPropertyChanged_ = false;
                    if (oldSession) {
                        currentPosition_ = 0;
                        currentDuration_ = 0;
                        currentStatus_ = GlobalSystemMediaTransportControlsSessionPlaybackStatus::Closed;
                        currentArtist_.clear();
                        currentTitle_.clear();
                        currentThumbnailPath_.clear();
                        currentThumbnailLength_ = 0;
                        if (infoUpdateCallback_) infoUpdateCallback_(currentArtist_, currentTitle_, currentThumbnailPath_);
                        if (progressUpdateCallback_) progressUpdateCallback_(currentPosition_, currentDuration_, currentStatus_);
                    }
                    continue;
                }
            }
            if (mediaPropertyChanged_) {
                mediaPropertyChanged_ = false;
                propertyChanged();
                getMediaProperties();
            }
            if (!currentSession_) {
                continue;
            }
            auto timelineProperties = currentSession_.GetTimelineProperties();
            auto playbackInfo = currentSession_.GetPlaybackInfo();
            auto status = playbackInfo.PlaybackStatus();
            if (status != currentStatus_) {
                currentStatus_ = status;
                progressDirty = true;
            }
            int64_t position = timelineProperties.Position().count();
            auto lastUpdatedTime = timelineProperties.LastUpdatedTime();
            if (lastUpdatedTime.time_since_epoch().count() > 0) {
                auto playbackRatePtr = playbackInfo.PlaybackRate();
                auto playbackRate = playbackRatePtr ? playbackRatePtr.Value() : 1.0;

                position += (int64_t)((DateTime::clock::now() - lastUpdatedTime).count() * playbackRate);
                int newPosition = (int)(position / DateTime::clock::period::den);
                if (newPosition != currentPosition_) {
                    currentPosition_ = newPosition;
                    progressDirty = true;
                }
                int newDuration = (int)(timelineProperties.EndTime().count() / DateTime::clock::period::den);
                if (newDuration != currentDuration_) {
                    currentDuration_ = newDuration;
                    progressDirty = true;
                }
            } else {
                if (currentPosition_ != 0 || currentDuration_ != 0) {
                    currentPosition_ = 0;
                    currentDuration_ = 0;
                    infoDirty_ = true;
                }
            }
            checkUpdateOfThumbnail();
        }
        if (infoDirty_ && infoUpdateCallback_) {
            infoUpdateCallback_(currentArtist_, currentTitle_, currentThumbnailPath_);
            infoDirty_ = false;
        }
        if (progressDirty && progressUpdateCallback_) {
            progressUpdateCallback_(currentPosition_, currentDuration_, currentStatus_);
        }
    }
}

void Smtc::stop() {
    isRunning_ = false;
}

void Smtc::getMediaProperties() {
    auto [status, newProperties] = WaitForAsyncOperation(currentSession_.TryGetMediaPropertiesAsync());
    if (status != AsyncStatus::Completed) return;
    std::lock_guard lock(sessionMutex_);
    currentProperties_ = newProperties;
    if (!currentProperties_) {
        if (!currentArtist_.empty() || !currentTitle_.empty()) {
            currentArtist_.clear();
            currentTitle_.clear();
            currentThumbnailPath_.clear();
            currentThumbnailLength_ = 0;
            infoDirty_ = true;
        }
        return;
    }
    auto str = currentProperties_.Artist();
    const std::wstring newArtist(str.begin(), str.end());
    str = currentProperties_.Title();
    const std::wstring newTitle(str.begin(), str.end());
    if (currentArtist_ != newArtist || currentTitle_ != newTitle) {
        currentArtist_ = newArtist;
        currentTitle_ = newTitle;
        infoDirty_ = true;
    }

    currentThumbnailLength_ = 0;
}

void Smtc::checkUpdateOfThumbnail() {
    bool wasNotEmpty = false;
    if (!currentThumbnailPath_.empty()) {
        wasNotEmpty = true;
    }
    do {
        auto thumbnail = currentProperties_.Thumbnail();
        if (!thumbnail) {
            break;
        }
        auto [status, stream] = WaitForAsyncOperation(thumbnail.OpenReadAsync());
        if (status != AsyncStatus::Completed) {
            break;
        }
        if (!stream) {
            break;
        }
        if (currentThumbnailLength_ == (int)stream.Size()) {
            return;
        }
        DataReader reader(stream.GetInputStreamAt(0));
        auto bufSize = stream.Size();
        auto [status2, bufLen] = WaitForAsyncOperation(reader.LoadAsync((uint32_t)bufSize));
        if (status2 != AsyncStatus::Completed) {
            break;
        }
        auto content = reader.ReadBuffer(bufLen);
        DWORD bytesWritten;
        HANDLE hFile;
        auto contentType = stream.ContentType();
        if (contentType == L"image/png") {
            hFile = CreateFileW(L"static/thumbnail.png", GENERIC_WRITE, 0, nullptr, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, nullptr);
            currentThumbnailPath_ = L"thumbnail.png";
        } else if (contentType.starts_with(L"image/jpeg") || contentType.starts_with(L"image/jpg")) {
            hFile = CreateFileW(L"static/thumbnail.jpg", GENERIC_WRITE, 0, nullptr, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, nullptr);
            currentThumbnailPath_ = L"thumbnail.jpg";
        } else {
            break;
        }
        if (hFile == INVALID_HANDLE_VALUE) {
            currentThumbnailPath_.clear();
            break;
        }
        WriteFile(hFile, content.data(), (DWORD)content.Length(), &bytesWritten, nullptr);
        CloseHandle(hFile);
        currentThumbnailLength_ = (int)bufSize;
        infoDirty_ = true;
        return;
    } while (false);

    currentThumbnailPath_.clear();
    currentThumbnailLength_ = 0;
    infoDirty_ = infoDirty_ || wasNotEmpty;
}
