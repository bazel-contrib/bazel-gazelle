package foo

import "embed"

//go:embed data.txt
var content embed.FS
