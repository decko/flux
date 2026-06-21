package api

import (
	"io/fs"
	"net/http"

	"github.com/decko/flux/web"
)

// spaFilesystem returns an http.FileSystem rooted at the embedded SPA build output.
func spaFilesystem() http.FileSystem {
	sub, err := fs.Sub(web.Files, "dist")
	if err != nil {
		panic("embed: web/dist not found - run 'make frontend' first")
	}
	return http.FS(sub)
}
