package foo

import "embed"

//go:embed a/b/deep.txt
var content embed.FS
