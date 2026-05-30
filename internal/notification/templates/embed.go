package templates

import (
	"embed"
	"io/fs"
)

//go:embed */meta.yaml */vi/*.txt */vi/*.html
var embeddedFiles embed.FS

func FS() fs.FS {
	return embeddedFiles
}
