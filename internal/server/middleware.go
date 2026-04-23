package server

import (
	"log/slog"
	"net/http"
	"time"
)

func localhostOnly(h http.HandlerFunc, allowRemote bool) http.HandlerFunc {
	if allowRemote {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLocalhost(r) {
			writeJSON(w, http.StatusForbidden, map[string]any{"success": false, "error": "forbidden"})
			return
		}
		h(w, r)
	}
}

func accessLog(next http.Handler, debug bool) http.Handler {
	if !debug {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(startedAt))
	})
}
