package foo

import (
	"embed"
	"testing"
)

//go:embed sub/shared.txt
var testContent embed.FS

func TestEmbed(t *testing.T) {}
