package foo

import "embed"

//go:embed sub/data.txt
var content embed.FS
