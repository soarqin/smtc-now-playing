// Package domain contains shared data types used across smtc-now-playing packages.
package domain

// PlaybackStatus represents the SMTC playback state
type PlaybackStatus int

const (
	StatusClosed   PlaybackStatus = 0
	StatusOpened   PlaybackStatus = 1
	StatusChanging PlaybackStatus = 2
	StatusStopped  PlaybackStatus = 3
	StatusPlaying  PlaybackStatus = 4
	StatusPaused   PlaybackStatus = 5
)

// PlaybackType represents the type of media being played
type PlaybackType int

const (
	PlaybackTypeUnknown PlaybackType = 0
	PlaybackTypeMusic   PlaybackType = 1
	PlaybackTypeVideo   PlaybackType = 2
	PlaybackTypeImage   PlaybackType = 3
)

// AutoRepeatMode represents the repeat mode
type AutoRepeatMode int

const (
	AutoRepeatNone  AutoRepeatMode = 0
	AutoRepeatTrack AutoRepeatMode = 1
	AutoRepeatList  AutoRepeatMode = 2
)

// InfoData holds media info for the current session
type InfoData struct {
	Artist               string
	Title                string
	ThumbnailContentType string
	ThumbnailData        []byte
	AlbumTitle           string
	AlbumArtist          string
	PlaybackType         int
	SourceApp            string
}

// Equal compares two InfoData structs for equality
func (i *InfoData) Equal(other *InfoData) bool {
	if other == nil {
		return false
	}
	return i.Artist == other.Artist &&
		i.Title == other.Title &&
		i.ThumbnailContentType == other.ThumbnailContentType &&
		len(i.ThumbnailData) == len(other.ThumbnailData) &&
		i.AlbumTitle == other.AlbumTitle &&
		i.AlbumArtist == other.AlbumArtist &&
		i.PlaybackType == other.PlaybackType &&
		i.SourceApp == other.SourceApp
}

// ProgressData holds playback progress for the current session
type ProgressData struct {
	Position        int
	Duration        int
	Status          int
	PlaybackRate    float64
	IsShuffleActive *bool
	AutoRepeatMode  int
	LastUpdatedTime int64
}

// Equal compares two ProgressData structs for equality
func (p *ProgressData) Equal(other *ProgressData) bool {
	if other == nil {
		return false
	}
	shuffleEqual := (p.IsShuffleActive == nil && other.IsShuffleActive == nil) ||
		(p.IsShuffleActive != nil && other.IsShuffleActive != nil && *p.IsShuffleActive == *other.IsShuffleActive)
	return p.Position == other.Position &&
		p.Duration == other.Duration &&
		p.Status == other.Status &&
		p.PlaybackRate == other.PlaybackRate &&
		shuffleEqual &&
		p.AutoRepeatMode == other.AutoRepeatMode &&
		p.LastUpdatedTime == other.LastUpdatedTime
}

// SessionInfo holds metadata about an available SMTC session
type SessionInfo struct {
	AppID       string
	Name        string
	SourceAppID string
}

// ControlCapabilities reports which media controls the current session supports
type ControlCapabilities struct {
	IsPlayEnabled     bool `json:"isPlayEnabled"`
	IsPauseEnabled    bool `json:"isPauseEnabled"`
	IsStopEnabled     bool `json:"isStopEnabled"`
	IsNextEnabled     bool `json:"isNextEnabled"`
	IsPreviousEnabled bool `json:"isPreviousEnabled"`
	IsSeekEnabled     bool `json:"isSeekEnabled"`
	IsShuffleEnabled  bool `json:"isShuffleEnabled"`
	IsRepeatEnabled   bool `json:"isRepeatEnabled"`
}

// Escape replicates C++ escape() — escapes special characters in artist/title strings
// Matches c/smtc.cpp:26-59 exactly
func Escape(s string) string {
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
