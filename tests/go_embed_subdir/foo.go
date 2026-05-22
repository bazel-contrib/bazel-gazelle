package foo

import "embed"

//go:embed static/*
var content embed.FS
