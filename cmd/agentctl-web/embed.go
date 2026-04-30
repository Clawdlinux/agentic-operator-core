package main

import (
	"embed"
	"io/fs"
)

//go:embed templates static
var embeddedFS embed.FS

// TemplatesFS returns the templates subtree.
func TemplatesFS() fs.FS {
	sub, _ := fs.Sub(embeddedFS, "templates")
	return sub
}

// StaticFS returns the static subtree for serving.
func StaticFS() fs.FS {
	sub, _ := fs.Sub(embeddedFS, "static")
	return sub
}
