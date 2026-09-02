package main

import (
	"encoding/json"
	"regexp"
)

const goListCommand = "go list -m -json all"

type testCase struct {
	Name       string                       `json:"name"`
	Modules    []module                     `json:"modules"`
	Files      map[string]string            `json:"files,omitempty"`
	Executions map[string]map[string]string `json:"executions,omitempty"`
	Want       map[string]json.RawMessage   `json:"want,omitempty"`
}

type module struct {
	Name         string `json:"name"`
	IsRoot       bool   `json:"is_root,omitempty"`
	Version      string `json:"version,omitempty"`
	Tags         *tags  `json:"tags,omitempty"`
	TagsDev      *tags  `json:"tags_dev,omitempty"`
	TagsIsolate  *tags  `json:"tags_isolate,omitempty"`
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

const mainExecutionKey = "main"

func isolateExecutionKey(moduleName string) string {
	return moduleName + "_isolate"
}

func goDepsWorkDir(moduleName string, isolated bool) string {
	if isolated {
		return "go_deps_" + moduleName + "_isolate"
	}
	return "go_deps"
}
