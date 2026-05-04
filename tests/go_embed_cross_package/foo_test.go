package foo

import (
	"embed"
	"testing"
)

//go:embed sub/data.txt
var testContent embed.FS

func TestEmbed(t *testing.T) {}
