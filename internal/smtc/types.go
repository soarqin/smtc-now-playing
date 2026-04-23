package smtc

import "smtc-now-playing/internal/domain"

// Status constants — match C++ GlobalSystemMediaTransportControlsSessionPlaybackStatus exactly
const (
	StatusClosed   = 0
	StatusOpened   = 1
	StatusChanging = 2
	StatusStopped  = 3
	StatusPlaying  = 4
	StatusPaused   = 5
)

// InfoData holds media info used internally for deduplication.
type InfoData struct {
	Artist               string
	Title                string
	ThumbnailContentType string
	ThumbnailData        []byte
	AlbumTitle           string
	AlbumArtist          string
	PlaybackType         int // 0=Unknown, 1=Music, 2=Video, 3=Image
	SourceApp            string
}

// ProgressData holds playback progress used internally for deduplication.
type ProgressData struct {
	Position        int
	Duration        int
	Status          int
	PlaybackRate    float64
	IsShuffleActive *bool // nil=unavailable, &true=on, &false=off
	AutoRepeatMode  int   // 0=None, 1=Track, 2=List
	LastUpdatedTime int64 // Unix milliseconds
}

// ControlCapabilities reports which media controls the current session supports.
type ControlCapabilities struct {
	IsPlayEnabled     bool `json:"isPlayEnabled"`
	IsPauseEnabled    bool `json:"isPauseEnabled"`
	IsStopEnabled     bool `json:"isStopEnabled"`
	IsNextEnabled     bool `json:"isNextEnabled"`
	IsPreviousEnabled bool `json:"isPreviousEnabled"`
	IsSeekEnabled     bool `json:"isSeekEnabled"` // position control
	IsShuffleEnabled  bool `json:"isShuffleEnabled"`
	IsRepeatEnabled   bool `json:"isRepeatEnabled"`
}

// SessionInfo holds metadata about an available SMTC session
type SessionInfo struct {
	AppID       string
	Name        string
	SourceAppID string
}

// Event is the sealed interface for all SMTC events. The unexported method
// prevents external types from implementing it.
type Event interface{ smtcEvent() }

// InfoEvent is emitted when media info changes.
type InfoEvent struct{ Data domain.InfoData }

func (InfoEvent) smtcEvent() {}

// ProgressEvent is emitted every ~200ms while media is active.
type ProgressEvent struct{ Data domain.ProgressData }

func (ProgressEvent) smtcEvent() {}

// SessionsChangedEvent is emitted when the set of SMTC sessions changes.
type SessionsChangedEvent struct{ Sessions []domain.SessionInfo }

func (SessionsChangedEvent) smtcEvent() {}

// DeviceChangedEvent is emitted when the active SMTC session switches.
type DeviceChangedEvent struct{ AppID string }

func (DeviceChangedEvent) smtcEvent() {}

// Options configures the Smtc instance.
type Options struct {
	InitialDevice string
}

func infoDataToDomain(data InfoData) domain.InfoData {
	thumb := append([]byte(nil), data.ThumbnailData...)
	return domain.InfoData{
		Artist:               data.Artist,
		Title:                data.Title,
		ThumbnailContentType: data.ThumbnailContentType,
		ThumbnailData:        thumb,
		AlbumTitle:           data.AlbumTitle,
		AlbumArtist:          data.AlbumArtist,
		PlaybackType:         data.PlaybackType,
		SourceApp:            data.SourceApp,
	}
}

func progressDataToDomain(data ProgressData) domain.ProgressData {
	var shuffle *bool
	if data.IsShuffleActive != nil {
		value := *data.IsShuffleActive
		shuffle = &value
	}
	return domain.ProgressData{
		Position:        data.Position,
		Duration:        data.Duration,
		Status:          data.Status,
		PlaybackRate:    data.PlaybackRate,
		IsShuffleActive: shuffle,
		AutoRepeatMode:  data.AutoRepeatMode,
		LastUpdatedTime: data.LastUpdatedTime,
	}
}

func sessionInfoToDomain(data SessionInfo) domain.SessionInfo {
	return domain.SessionInfo{
		AppID:       data.AppID,
		Name:        data.Name,
		SourceAppID: data.SourceAppID,
	}
}

func sessionInfosToDomain(data []SessionInfo) []domain.SessionInfo {
	if len(data) == 0 {
		return nil
	}
	out := make([]domain.SessionInfo, len(data))
	for i, item := range data {
		out[i] = sessionInfoToDomain(item)
	}
	return out
}

// escape replicates C++ escape() — escapes special characters in artist/title strings
// Matches c/smtc.cpp:26-59 exactly
func escape(s string) string {
	var result []rune
	for _, c := range s {
		switch c {
		case '\n':
			result = append(result, '\\', 'n')
		case '\r':
			result = append(result, '\\', 'r')
		case '\t':
			result = append(result, '\\', 't')
		case '\\':
			result = append(result, '\\', '\\')
		case '\v':
			result = append(result, '\\', 'v')
		case '\b':
			result = append(result, '\\', 'b')
		case '\f':
			result = append(result, '\\', 'f')
		case '\a':
			result = append(result, '\\', 'a')
		default:
			result = append(result, c)
		}
	}
	return string(result)
}
