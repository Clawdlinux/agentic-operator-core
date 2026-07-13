// Package webtheme provides the shared Clawdlinux web theme assets.
package webtheme

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var embeddedFS embed.FS

// FS returns the embedded assets directory as the filesystem root.
func FS() fs.FS {
	sub, _ := fs.Sub(embeddedFS, "assets")
	return sub
}
