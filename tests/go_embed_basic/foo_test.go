package foo

import (
	"embed"
	"testing"
)

//go:embed data.txt
var testContent embed.FS

func TestEmbed(t *testing.T) {}
