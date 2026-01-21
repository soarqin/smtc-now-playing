package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type WebServer struct {
	httpSrv *http.Server
	smtc    *Smtc

	currentTheme string

	currentMutex    sync.Mutex
	currentInfo     string
	currentProgress string

	errorChan            chan error
	quitChan             chan struct{}
	waitGroup            sync.WaitGroup
	infoUpdate           []chan string
	progressUpdate       []chan string
	infoChannelMutex     sync.Mutex
	progressChannelMutex sync.Mutex
	albumArtContentType  string
	albumArtData         []byte
}

type infoDetail struct {
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	AlbumArt string `json:"albumArt"`
}

type progressDetail struct {
	Position int `json:"position"`
	Duration int `json:"duration"`
	Status   int `json:"status"`
}

func NewWebServer(host string, port string, theme string) *WebServer {
	smtc := SmtcCreate()
	if smtc.Init() != 0 {
		return nil
	}

	mux := http.NewServeMux()
	srv := &WebServer{
		httpSrv: &http.Server{
			Addr:    fmt.Sprintf("%s:%s", host, port),
			Handler: mux,
		},
		smtc: smtc,

		currentTheme: theme,

		infoUpdate:     make([]chan string, 0),
		progressUpdate: make([]chan string, 0),
	}

	mux.HandleFunc("/update_event", srv.handleUpdateEvent)
	mux.HandleFunc("/albumArt/", srv.handleAlbumArt)
	mux.HandleFunc("/script/", srv.handleScript)
	mux.HandleFunc("/", srv.handleStatic)

	return srv
}

func (srv *WebServer) addInfoUpdateChannel() chan string {
	srv.infoChannelMutex.Lock()
	defer srv.infoChannelMutex.Unlock()
	ch := make(chan string, 1)
	srv.infoUpdate = append(srv.infoUpdate, ch)
	return ch
}

func (srv *WebServer) removeInfoUpdateChannel(ch chan string) {
	srv.infoChannelMutex.Lock()
	defer srv.infoChannelMutex.Unlock()
	for i, c := range srv.infoUpdate {
		if c == ch {
			srv.infoUpdate = append(srv.infoUpdate[:i], srv.infoUpdate[i+1:]...)
		}
	}
}

func (srv *WebServer) addProgressUpdateChannel() chan string {
	srv.progressChannelMutex.Lock()
	defer srv.progressChannelMutex.Unlock()
	ch := make(chan string, 1)
	srv.progressUpdate = append(srv.progressUpdate, ch)
	return ch
}

func (srv *WebServer) removeProgressUpdateChannel(ch chan string) {
	srv.progressChannelMutex.Lock()
	defer srv.progressChannelMutex.Unlock()
	for i, c := range srv.progressUpdate {
		if c == ch {
			srv.progressUpdate = append(srv.progressUpdate[:i], srv.progressUpdate[i+1:]...)
		}
	}
}

func writeSSEHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func writeSSEData(w http.ResponseWriter, rc *http.ResponseController, data string) error {
	_, err := fmt.Fprintf(w, "data: %v\n\n", data)
	if err != nil {
		return err
	}
	err = rc.Flush()
	if err != nil {
		return err
	}
	return nil
}

func (srv *WebServer) handleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	writeSSEHeader(w)

	rc := http.NewResponseController(w)

	srv.currentMutex.Lock()
	info := srv.currentInfo
	progress := srv.currentProgress
	srv.currentMutex.Unlock()
	err := writeSSEData(w, rc, info)
	if err != nil {
		return
	}
	err = writeSSEData(w, rc, progress)
	if err != nil {
		return
	}

	ch := srv.addInfoUpdateChannel()
	defer srv.removeInfoUpdateChannel(ch)
	ch2 := srv.addProgressUpdateChannel()
	defer srv.removeProgressUpdateChannel(ch2)

	clientGone := r.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			err := writeSSEData(w, rc, data)
			if err != nil {
				return
			}
		case data, ok := <-ch2:
			if !ok {
				return
			}
			err := writeSSEData(w, rc, data)
			if err != nil {
				return
			}
		}
	}
}

func (srv *WebServer) handleAlbumArt(w http.ResponseWriter, r *http.Request) {
	if len(srv.albumArtData) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", srv.albumArtContentType)
	w.Write(srv.albumArtData)
}

func (srv *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, fmt.Sprintf("themes/%s/%s", srv.currentTheme, r.URL.Path[1:]))
}

func (srv *WebServer) handleScript(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
}

func (srv *WebServer) Start() {
	srv.errorChan = make(chan error, 1)
	srv.quitChan = make(chan struct{}, 1)
	srv.waitGroup.Add(2)
	go func() {
		err := srv.httpSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			srv.errorChan <- err
		}
		srv.waitGroup.Done()
	}()
	go func() {
		var info infoDetail
		var progress progressDetail

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-srv.quitChan:
				srv.waitGroup.Done()
				return
			case <-ticker.C:
				srv.smtc.Update()
				dirty := srv.smtc.RetrieveDirtyData(&info.Artist, &info.Title, &srv.albumArtContentType, &srv.albumArtData, &progress.Position, &progress.Duration, &progress.Status)
				if dirty&1 != 0 {
					if len(srv.albumArtData) > 0 {
						checksum := sha256.Sum256(srv.albumArtData)
						info.AlbumArt = "/albumArt/" + hex.EncodeToString(checksum[:])
					} else {
						info.AlbumArt = ""
					}
					j, err := json.Marshal(&struct {
						Type string      `json:"type"`
						Data *infoDetail `json:"data"`
					}{
						Type: "info",
						Data: &info,
					})
					if err == nil {
						info := string(j)
						srv.currentMutex.Lock()
						if info != srv.currentInfo {
							srv.currentInfo = info
							srv.currentMutex.Unlock()
							for _, ch := range srv.infoUpdate {
								ch <- info
							}
						} else {
							srv.currentMutex.Unlock()
						}
					}
				}
				if dirty&2 != 0 {
					j, err := json.Marshal(&struct {
						Type string          `json:"type"`
						Data *progressDetail `json:"data"`
					}{
						Type: "progress",
						Data: &progress,
					})
					if err == nil {
						progress := string(j)
						srv.currentMutex.Lock()
						if progress != srv.currentProgress {
							srv.currentProgress = progress
							srv.currentMutex.Unlock()
							for _, ch := range srv.progressUpdate {
								ch <- progress
							}
						} else {
							srv.currentMutex.Unlock()
						}
					}
				}
			}
		}
	}()
}

func (srv *WebServer) Stop() {
	srv.currentInfo = ""
	srv.currentProgress = ""
	for _, ch := range srv.infoUpdate {
		close(ch)
	}
	for _, ch := range srv.progressUpdate {
		close(ch)
	}
	close(srv.quitChan)
	srv.httpSrv.Close()
	srv.waitGroup.Wait()
	srv.smtc.Destroy()
}

func (srv *WebServer) Address() string {
	return srv.httpSrv.Addr
}

func (srv *WebServer) Error() <-chan error {
	return srv.errorChan
}

func (srv *WebServer) SetTheme(theme string) {
	srv.currentTheme = theme
}

// func unescape(str string) string {
// 	result := strings.Builder{}
// 	l := len(str)
// 	for i := 0; i < l; i++ {
// 		c := str[i]
// 		if c == '\\' {
// 			i++
// 			if i >= l {
// 				break
// 			}
// 			switch str[i] {
// 			case 'n':
// 				result.WriteRune('\n')
// 			case 'r':
// 				result.WriteRune('\r')
// 			case 't':
// 				result.WriteRune('\t')
// 			case 'v':
// 				result.WriteRune('\v')
// 			case 'b':
// 				result.WriteRune('\b')
// 			case 'f':
// 				result.WriteRune('\f')
// 			case 'a':
// 				result.WriteRune('\a')
// 			default:
// 				result.WriteByte(c)
// 			}
// 		} else {
// 			result.WriteByte(c)
// 		}
// 	}
// 	return result.String()
// }
