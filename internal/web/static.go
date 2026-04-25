package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded web/dist filesystem rooted at "dist/".
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // can only fail if "dist" doesn't exist in embed
	}
	return sub
}
