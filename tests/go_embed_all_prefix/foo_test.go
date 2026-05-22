package foo

import (
	"embed"
	"testing"
)

//go:embed all:dir
var testContent embed.FS

func TestEmbed(t *testing.T) {}
