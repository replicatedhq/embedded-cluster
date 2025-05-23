package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var static embed.FS

var staticFS fs.FS

func init() {
	var err error
	staticFS, err = fs.Sub(static, "dist")
	if err != nil {
		panic(err)
	}
}

func Fs() fs.FS {
	return staticFS
}
