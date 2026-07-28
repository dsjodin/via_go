// Package webui embeds the built frontend into the binary.
//
// The contents of dist/ are produced by building the Next.js application in
// ui/ (see the go:generate directive in main.go). A placeholder index.html is
// committed so that the Go build works without a Node toolchain present; a
// real build overwrites it.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// FS returns the embedded frontend rooted at the build output directory.
func FS() (fs.FS, error) {
	return fs.Sub(files, "dist")
}
