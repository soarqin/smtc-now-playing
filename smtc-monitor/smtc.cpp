#include "smtc.h"

#include <winrt/Windows.Storage.Streams.h>
#include <winrt/Windows.Foundation.h>
#include <windows.h>
#include <shlwapi.h>

#include <chrono>
#include <tuple>
#include <mutex>
#include <sstream>

using namespace Windows::Foundation;
using namespace Windows::Storage::Streams;

Smtc::Smtc() {
    init_apartment();
}

Smtc::~Smtc() {
    uninit_apartment();
}

static std::wstring escape(const std::wstring& str) {
    std::wostringstream result;
    for (auto c : str) {
        switch (c) {
        case L'\n':
            result << L"\\n";
            break;
        case L'\r':
            result << L"\\r";
            break;
        case L'\t':
            result << L"\\t";
            break;
        case L'\\':
            result << L"\\\\";
            break;
        case L'\v':
            result << L"\\v";
            break;
        case L'\b':
            result << L"\\b";
            break;
        case L'\f':
            result << L"\\f";
            break;
        case L'\a':
            result << L"\\a";
            break;
        default:
            result << c;
        }
    }
    return result.str();
}

struct HandleHolder {
    HANDLE hEvent = nullptr;
    HandleHolder(HANDLE hEvent = nullptr) : hEvent(hEvent) {}
    HandleHolder(const HandleHolder&) = delete;
    HandleHolder& operator=(const HandleHolder&) = delete;
    HandleHolder(HandleHolder&& other) noexcept : hEvent(other.hEvent) {
        other.hEvent = nullptr;
    }
    HandleHolder& operator=(HandleHolder&& other) noexcept {
        if (this->hEvent) {
            CloseHandle(this->hEvent);
        }
        hEvent = other.hEvent;
        other.hEvent = nullptr;
        return *this;
    }
    HandleHolder& operator=(HANDLE hEvent) {
        if (this->hEvent) {
            CloseHandle(this->hEvent);
        }
        this->hEvent = hEvent;
        return *this;
    }
    ~HandleHolder() {
        if (hEvent) {
            CloseHandle(hEvent);
            hEvent = nullptr;
        }
    }
    operator HANDLE() const {
        return hEvent;
    }
};

template<typename T>
inline static std::tuple<AsyncStatus, T> WaitForAsyncOperation(IAsyncOperation<T> operation) {
    static thread_local std::vector<HandleHolder> hEvents;

    if (operation.Status() != AsyncStatus::Completed) {
        HandleHolder event(nullptr);
        if (hEvents.empty()) {
            event = CreateEventW(nullptr, FALSE, FALSE, nullptr);
        } else {
            event = std::move(hEvents.back());
            hEvents.pop_back();
        }
        operation.Completed([&](const IAsyncOperation<T>& sender, const AsyncStatus& status) {
            SetEvent(event);
        });
        WaitForSingleObject(event, INFINITE);

        if (hEvents.size() < 16) {
            hEvents.emplace_back(std::move(event));
        }
    }
    auto status = operation.Status();
    if (status == AsyncStatus::Completed) {
        return std::make_tuple(status, operation.GetResults());
    }
    return std::make_tuple(status, T(0));
}

template<typename T>
inline static AsyncStatus WaitForAsyncOperationNoReturn(IAsyncOperation<T> operation) {
    if (operation.Status() != AsyncStatus::Completed) {
        HANDLE hEvent = CreateEventW(nullptr, TRUE, FALSE, nullptr);
        operation.Completed([&](const IAsyncOperation<T>& sender, const AsyncStatus& status) {
            SetEvent(hEvent);
        });
        WaitForSingleObject(hEvent, INFINITE);
        CloseHandle(hEvent);
    }
    return operation.Status();
}

int Smtc::init() {
    auto [status, sessionManager] = WaitForAsyncOperation(GlobalSystemMediaTransportControlsSessionManager::RequestAsync());
    if (status != AsyncStatus::Completed) {
        return -1;
    }
    sessionManager_ = sessionManager;
    currentSession_ = sessionManager_.GetCurrentSession();
    mediaPropertyChanged_.store(currentSession_ != nullptr);
    if (currentSession_) propertyChanged();
    sessionManager_.CurrentSessionChanged([&](const GlobalSystemMediaTransportControlsSessionManager& sender, const CurrentSessionChangedEventArgs& args) {
        mediaChanged_.store(true);
        mediaPropertyChanged_.store(true);
    });
    return 0;
}

void Smtc::update() {
    if (mediaChanged_.exchange(false)) {
        std::lock_guard lock(sessionMutex_);
        currentProperties_ = nullptr;
        auto oldSession = currentSession_;
        currentSession_ = sessionManager_.GetCurrentSession();
        if (!currentSession_) {
            mediaPropertyChanged_.store(false);
            if (oldSession) {
                currentPosition_ = 0;
                currentDuration_ = 0;
                currentStatus_ = GlobalSystemMediaTransportControlsSessionPlaybackStatus::Closed;
                currentArtist_.clear();
                currentTitle_.clear();
                currentThumbnailPath_.clear();
                currentThumbnailLength_ = 0;
                infoDirty_.store(true);
                progressDirty_.store(true);
            }
            return;
        }
    }
    if (mediaPropertyChanged_.exchange(false)) {
        propertyChanged();
        getMediaProperties();
    }
    if (!currentSession_) {
        return;
    }
    auto timelineProperties = currentSession_.GetTimelineProperties();
    auto playbackInfo = currentSession_.GetPlaybackInfo();
    auto status = playbackInfo.PlaybackStatus();
    if (status != currentStatus_) {
        currentStatus_ = status;
        progressDirty_.store(true);
    }
    int64_t position = timelineProperties.Position().count();
    auto lastUpdatedTime = timelineProperties.LastUpdatedTime();
    if (lastUpdatedTime.time_since_epoch().count() > 0) {
        if (status == GlobalSystemMediaTransportControlsSessionPlaybackStatus::Playing) {
            auto playbackRatePtr = playbackInfo.PlaybackRate();
            auto playbackRate = playbackRatePtr ? playbackRatePtr.Value() : 1.0;

            position += (int64_t)((DateTime::clock::now() - lastUpdatedTime).count() * playbackRate);
        }
        int newPosition = (int)(position / DateTime::clock::period::den);
        if (newPosition != currentPosition_) {
            currentPosition_ = newPosition;
            progressDirty_.store(true);
        }
        int newDuration = (int)(timelineProperties.EndTime().count() / DateTime::clock::period::den);
        if (newDuration != currentDuration_) {
            currentDuration_ = newDuration;
            progressDirty_.store(true);
        }
    } else {
        if (currentPosition_ != 0 || currentDuration_ != 0) {
            currentPosition_ = 0;
            currentDuration_ = 0;
            progressDirty_.store(true);
        }
    }

    std::lock_guard lock(sessionMutex_);
    checkUpdateOfThumbnail();
}

int Smtc::retrieveDirtyData(wchar_t *artist, wchar_t *title, wchar_t *thumbnailPath, int *position, int *duration, int *status) {
    std::lock_guard lock(sessionMutex_);
    int dirty = 0;
    if (infoDirty_.exchange(false)) {
        StrCpyNW(artist, currentArtist_.c_str(), 256);
        StrCpyNW(title, currentTitle_.c_str(), 256);
        StrCpyNW(thumbnailPath, currentThumbnailPath_.c_str(), 1024);
        dirty |= 1;
    }
    if (progressDirty_.exchange(false)) {
        *position = currentPosition_;
        *duration = currentDuration_;
        *status = (int)currentStatus_;
        dirty |= 2;
    }
    return dirty;
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
            infoDirty_.store(true);
        }
        return;
    }
    auto str = currentProperties_.Artist();
    const std::wstring newArtist(str.begin(), str.end());
    str = currentProperties_.Title();
    const std::wstring newTitle(str.begin(), str.end());
    if (currentArtist_ != newArtist || currentTitle_ != newTitle) {
        currentArtist_ = escape(newArtist);
        currentTitle_ = escape(newTitle);
        infoDirty_.store(true);
    }

    currentThumbnailLength_ = 0;
}

void Smtc::checkUpdateOfThumbnail() {
    bool wasNotEmpty = false;
    if (!currentThumbnailPath_.empty()) {
        wasNotEmpty = true;
    }
    do {
        if (!currentProperties_) {
            break;
        }
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
            currentThumbnailPath_ = L"image/thumbnail.png";
        } else if (contentType.starts_with(L"image/jpeg") || contentType.starts_with(L"image/jpg")) {
            currentThumbnailPath_ = L"image/thumbnail.jpg";
        } else {
            break;
        }
        hFile = CreateFileW(currentThumbnailPath_.c_str(), GENERIC_WRITE, 0, nullptr, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, nullptr);
        if (hFile == INVALID_HANDLE_VALUE) {
            currentThumbnailPath_.clear();
            break;
        }
        WriteFile(hFile, content.data(), (DWORD)content.Length(), &bytesWritten, nullptr);
        CloseHandle(hFile);
        currentThumbnailLength_ = (int)bufSize;
        infoDirty_.store(true);
        return;
    } while (false);

    currentThumbnailPath_.clear();
    currentThumbnailLength_ = 0;
    infoDirty_.store(infoDirty_.load() || wasNotEmpty);
}

void Smtc::propertyChanged() {
    currentSession_.MediaPropertiesChanged([&](const GlobalSystemMediaTransportControlsSession& sender, const MediaPropertiesChangedEventArgs& args) {
        mediaPropertyChanged_.store(true);
    });
}
