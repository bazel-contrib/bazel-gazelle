package foo

import "embed"

//go:embed sub/shared.txt
var content embed.FS
