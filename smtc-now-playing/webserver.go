package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type WebServer struct {
	httpSrv *http.Server
	monitor *Monitor

	currentTheme string

	currentMutex    sync.Mutex
	currentInfo     string
	currentProgress string

	errorChan            chan error
	infoUpdate           []chan string
	progressUpdate       []chan string
	infoChannelMutex     sync.Mutex
	progressChannelMutex sync.Mutex
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

func NewWebServer(host string, port string, monitor *Monitor, theme string) *WebServer {
	mux := http.NewServeMux()
	srv := &WebServer{
		httpSrv: &http.Server{
			Addr:    fmt.Sprintf("%s:%s", host, port),
			Handler: mux,
		},
		monitor: monitor,

		currentTheme: theme,

		infoUpdate:     make([]chan string, 0),
		progressUpdate: make([]chan string, 0),
	}

	mux.HandleFunc("/info_changed", srv.handleInfoChanged)
	mux.HandleFunc("/progress_changed", srv.handleProgressChanged)
	mux.HandleFunc("/image/", srv.handleImage)
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

func (srv *WebServer) handleInfoChanged(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := srv.addInfoUpdateChannel()

	clientGone := r.Context().Done()
	rc := http.NewResponseController(w)
	defer srv.removeInfoUpdateChannel(ch)

	srv.currentMutex.Lock()
	info := srv.currentInfo
	srv.currentMutex.Unlock()
	_, err := fmt.Fprintf(w, "data: %v\n\n", info)
	if err != nil {
		return
	}
	err = rc.Flush()
	if err != nil {
		return
	}

	for {
		select {
		case <-clientGone:
			return
		case update, ok := <-ch:
			if !ok {
				return
			}
			_, err := fmt.Fprintf(w, "data: %v\n\n", update)
			if err != nil {
				return
			}
			err = rc.Flush()
			if err != nil {
				return
			}
		}
	}
}

func (srv *WebServer) handleProgressChanged(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := srv.addProgressUpdateChannel()

	clientGone := r.Context().Done()
	rc := http.NewResponseController(w)
	defer srv.removeProgressUpdateChannel(ch)

	srv.currentMutex.Lock()
	progress := srv.currentProgress
	srv.currentMutex.Unlock()
	_, err := fmt.Fprintf(w, "data: %v\n\n", progress)
	if err != nil {
		return
	}
	err = rc.Flush()
	if err != nil {
		return
	}

	for {
		select {
		case <-clientGone:
			return
		case update, ok := <-ch:
			if !ok {
				return
			}
			_, err := fmt.Fprintf(w, "data: %v\n\n", update)
			if err != nil {
				return
			}
			err = rc.Flush()
			if err != nil {
				return
			}
		}
	}
}

func (srv *WebServer) handleImage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
}

func (srv *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, fmt.Sprintf("themes/%s/%s", srv.currentTheme, r.URL.Path[1:]))
}

func (srv *WebServer) handleScript(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, fmt.Sprintf("%s", r.URL.Path[1:]))
}

func (srv *WebServer) Start() {
	srv.errorChan = make(chan error, 1)
	go func() {
		err := srv.httpSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			srv.errorChan <- err
		}
	}()
	go func() {
		infoUpdate := srv.monitor.GetOutputChannel()
		for update := range infoUpdate {
			parts := strings.Split(update, "\t")
			partCount := len(parts)
			if partCount < 1 {
				continue
			}
			switch parts[0] {
			case "I":
				if partCount < 4 {
					break
				}
				j, err := json.Marshal(&infoDetail{
					Artist:   unescape(parts[1]),
					Title:    unescape(parts[2]),
					AlbumArt: unescape(parts[3]),
				})
				if err != nil {
					continue
				}
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
			case "P":
				if partCount < 4 {
					break
				}
				position, err := strconv.Atoi(parts[1])
				if err != nil {
					continue
				}
				duration, err := strconv.Atoi(parts[2])
				if err != nil {
					continue
				}
				status, err := strconv.Atoi(parts[3])
				if err != nil {
					continue
				}
				j, err := json.Marshal(&progressDetail{
					Position: position,
					Duration: duration,
					Status:   status,
				})
				if err != nil {
					continue
				}
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
	}()
}

func (srv *WebServer) Stop() {
	for _, ch := range srv.infoUpdate {
		close(ch)
	}
	for _, ch := range srv.progressUpdate {
		close(ch)
	}
	srv.httpSrv.Close()
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

func unescape(str string) string {
	result := strings.Builder{}
	l := len(str)
	for i := 0; i < l; i++ {
		c := str[i]
		if c == '\\' {
			i++
			if i >= l {
				break
			}
			switch str[i] {
			case 'n':
				result.WriteRune('\n')
			case 'r':
				result.WriteRune('\r')
			case 't':
				result.WriteRune('\t')
			case 'v':
				result.WriteRune('\v')
			case 'b':
				result.WriteRune('\b')
			case 'f':
				result.WriteRune('\f')
			case 'a':
				result.WriteRune('\a')
			default:
				result.WriteByte(c)
			}
		} else {
			result.WriteByte(c)
		}
	}
	return result.String()
}
