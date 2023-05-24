package server

import "net/http"

func serveStatic(urlPrefix, staticDir string) http.Handler {
	return http.StripPrefix(urlPrefix, http.FileServer(http.Dir(staticDir)))
}
