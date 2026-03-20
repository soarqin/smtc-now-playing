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
}

// ProgressData holds playback progress passed to OnProgress callback
type ProgressData struct {
	Position int
	Duration int
	Status   int
}

// InfoCallback is called when media info changes
type InfoCallback func(InfoData)

// ProgressCallback is called when playback progress changes
type ProgressCallback func(ProgressData)

// SessionInfo holds metadata about an available SMTC session
type SessionInfo struct {
	AppID string
	Name  string
}

// Options configures the Smtc instance
type Options struct {
	OnInfo                 InfoCallback
	OnProgress             ProgressCallback
	OnSessionsChanged      func([]SessionInfo)
	OnSelectedDeviceChange func(string)
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
