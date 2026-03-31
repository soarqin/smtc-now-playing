package smtc

// Status constants — match C++ GlobalSystemMediaTransportControlsSessionPlaybackStatus exactly
const (
	StatusClosed   = 0
	StatusOpened   = 1
	StatusChanging = 2
	StatusStopped  = 3
	StatusPlaying  = 4
	StatusPaused   = 5
)

// InfoData holds media info passed to OnInfo callback
type InfoData struct {
	Artist               string
	Title                string
	ThumbnailContentType string
	ThumbnailData        []byte
	AlbumTitle           string `json:"albumTitle"`
	AlbumArtist          string `json:"albumArtist"`
	PlaybackType         int    `json:"playbackType"` // 0=Unknown, 1=Music, 2=Video, 3=Image
}

// ProgressData holds playback progress passed to OnProgress callback
type ProgressData struct {
	Position        int
	Duration        int
	Status          int
	PlaybackRate    float64 `json:"playbackRate"`
	IsShuffleActive *bool   `json:"isShuffleActive"` // nil=unavailable, &true=on, &false=off
	AutoRepeatMode  int     `json:"autoRepeatMode"`  // 0=None, 1=Track, 2=List
	LastUpdatedTime int64   `json:"lastUpdatedTime"` // Unix milliseconds
}

// InfoCallback is called when media info changes
type InfoCallback func(InfoData)

// ProgressCallback is called when playback progress changes
type ProgressCallback func(ProgressData)

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
	SourceAppId string
}

// Options configures the Smtc instance
type Options struct {
	OnInfo                 InfoCallback
	OnProgress             ProgressCallback
	OnSessionsChanged      func([]SessionInfo)
	OnSelectedDeviceChange func(string)
	InitialDevice          string
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
