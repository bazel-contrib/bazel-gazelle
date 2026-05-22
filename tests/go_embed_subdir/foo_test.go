package foo

import (
	"embed"
	"testing"
)

//go:embed static/*
var testContent embed.FS

func TestEmbed(t *testing.T) {}
