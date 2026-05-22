package foo

import (
	"embed"
	"testing"
)

//go:embed dir
var testContent embed.FS

func TestEmbed(t *testing.T) {}
