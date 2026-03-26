package adminui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedFiles embed.FS

var embeddedDistFS = mustSubFS(embeddedFiles, "dist")

func mustSubFS(filesystem fs.FS, dir string) fs.FS {
	subtree, err := fs.Sub(filesystem, dir)
	if err != nil {
		panic(err)
	}
	return subtree
}
