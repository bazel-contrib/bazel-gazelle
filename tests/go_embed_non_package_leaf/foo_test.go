package foo

import (
	"embed"
	"testing"
)

//go:embed a/b/c/data.txt
var testContent embed.FS

func TestEmbed(t *testing.T) {}
