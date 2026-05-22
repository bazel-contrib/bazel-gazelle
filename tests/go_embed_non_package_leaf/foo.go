package foo

import "embed"

//go:embed a/b/c/data.txt
var content embed.FS
