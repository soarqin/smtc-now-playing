package server

import "net/http"

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
