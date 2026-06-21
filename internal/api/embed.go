package api

import (
	"io/fs"
	"net/http"

	"github.com/decko/flux/web"
)

// spaFS returns the embedded SPA filesystem as an fs.FS for file existence checks.
func spaFS() fs.FS {
	sub, err := fs.Sub(web.Files, "dist")
	if err != nil {
		panic("embed: web/dist not found - run 'make frontend' first")
	}
	return sub
}

// spaFilesystem returns an http.FileSystem rooted at the embedded SPA build output.
func spaFilesystem() http.FileSystem {
	return http.FS(spaFS())
}
