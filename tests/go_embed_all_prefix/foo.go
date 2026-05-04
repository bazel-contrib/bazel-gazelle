package foo

import "embed"

//go:embed all:dir
var content embed.FS
