package api

import (
	"net/http"
	"os"
	"path/filepath"
)

// SPAHandler serves a built single-page app (web/dist) with client-side
// routing support: any GET that doesn't match a real file under dir falls
// back to index.html, so React Router's /datasets, /query, etc. resolve on
// a hard refresh instead of 404ing (cahier des charges M1.6: "build
// statique servi par Go").
func SPAHandler(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	indexPath := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		full := filepath.Join(dir, clean)

		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			http.ServeFile(w, r, indexPath)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// staticDirAvailable reports whether dir looks like a real built frontend
// (contains index.html), so bootstrap can skip mounting it in dev setups
// where `npm run build` hasn't been run yet rather than serving 404s for
// every route.
func staticDirAvailable(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil
}
