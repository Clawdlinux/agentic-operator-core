// Package webtheme provides the shared Clawdlinux web theme assets.
package webtheme

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var embeddedFS embed.FS

var assetsFS = mustSub(embeddedFS, "assets")

// FS returns the embedded assets directory as the filesystem root.
func FS() fs.FS {
	return assetsFS
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("webtheme: invalid embedded asset layout: " + err.Error())
	}
	return sub
}
