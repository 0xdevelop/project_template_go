package api_common

import "net/http"

// HomeHandler returns a neutral root response for HTTP API listeners.
func HomeHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// RobotsHandler prevents compliant crawlers from indexing API listeners.
func RobotsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
}
