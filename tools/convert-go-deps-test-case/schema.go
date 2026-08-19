package main

import (
	"encoding/json"
	"regexp"
)

type testCase struct {
	Name    string            `json:"name"`
	Modules []module          `json:"modules"`
	Files   map[string]string `json:"files,omitempty"`
	Want    json.RawMessage   `json:"want,omitempty"`
}

type module struct {
	Name    string `json:"name"`
	IsRoot  bool   `json:"is_root,omitempty"`
	Version string `json:"version,omitempty"`
	Tags    *tags  `json:"tags,omitempty"`
}

type tags struct {
	ArchiveOverride          []map[string]any `json:"archive_override,omitempty"`
	Config                   []map[string]any `json:"config,omitempty"`
	FromFile                 []map[string]any `json:"from_file,omitempty"`
	GazelleOverride          []map[string]any `json:"gazelle_override,omitempty"`
	GazelleDefaultAttributes []map[string]any `json:"gazelle_default_attributes,omitempty"`
	Module                   []map[string]any `json:"module,omitempty"`
	ModuleOverride           []map[string]any `json:"module_override,omitempty"`
}

var labelRE = regexp.MustCompile(`^@+([^/]+)//(.*)$`)
