/* Copyright 2026 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package sh is a reference Gazelle plugin for shell scripts, written with
// package [github.com/bazel-contrib/bazel-gazelle/v2/plugin/server]. The same
// plugin is implemented in Python, without any dependencies, in sh_plugin.py.
//
// For each *.sh file in a directory, the plugin generates an sh_test if the
// file name ends with _test.sh, an sh_binary if the file starts with "#!",
// and an sh_library otherwise. A script's dependencies are the libraries
// that provide the files it sources with "source path" or ". path". Paths
// starting with ./ or ../ are relative to the script's directory; other paths
// are relative to the repository root. Paths containing variables are
// ignored.
//
// The directive "# gazelle:sh_enabled false" disables rule generation in a
// directory and its subdirectories.
package sh

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol"
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/server"
)

const (
	langName         = "sh"
	enabledDirective = "sh_enabled"
)

// Plugin implements the shell plugin.
type Plugin struct{}

// New returns a new shell plugin.
func New() *Plugin { return &Plugin{} }

var (
	_ server.Plugin     = (*Plugin)(nil)
	_ server.Configurer = (*Plugin)(nil)
	_ server.Generator  = (*Plugin)(nil)
	_ server.Indexer    = (*Plugin)(nil)
	_ server.Resolver   = (*Plugin)(nil)
)

// shConfig is the plugin's per-directory configuration. Gazelle stores it
// and sends it back with each request for the directory.
type shConfig struct {
	Enabled bool `json:"enabled"`
}

func parseConfig(raw json.RawMessage) (shConfig, error) {
	cfg := shConfig{Enabled: true}
	if len(raw) == 0 || string(raw) == "null" {
		return cfg, nil
	}
	err := json.Unmarshal(raw, &cfg)
	return cfg, err
}

func kind(name, bzl string) protocol.KindInfo {
	return protocol.KindInfo{
		Name:           name,
		LoadedFrom:     "@rules_shell//shell:" + bzl,
		NonEmptyAttrs:  []string{"srcs", "deps"},
		MergeableAttrs: []string{"srcs"},
		ResolveAttrs:   []string{"deps"},
	}
}

var kinds = []protocol.KindInfo{
	kind("sh_binary", "sh_binary.bzl"),
	kind("sh_library", "sh_library.bzl"),
	kind("sh_test", "sh_test.bzl"),
}

func isShKind(k string) bool {
	return slices.ContainsFunc(kinds, func(ki protocol.KindInfo) bool { return ki.Name == k })
}

// Initialize implements [server.Plugin].
func (*Plugin) Initialize(ctx context.Context, p protocol.InitializeParams) (protocol.InitializeResult, error) {
	return protocol.InitializeResult{
		ProtocolVersion: protocol.Version,
		Name:            langName,
		Kinds:           kinds,
		KnownDirectives: []string{enabledDirective},
		Capabilities: protocol.Capabilities{
			// Configure only needs to run where the sh_enabled directive appears.
			Configure: protocol.ConfigureDirectives,
		},
	}, nil
}

// Configure implements [server.Configurer].
func (*Plugin) Configure(ctx context.Context, p protocol.ConfigureParams) (protocol.ConfigureResult, error) {
	cfg, err := parseConfig(p.Config)
	if err != nil {
		return protocol.ConfigureResult{}, err
	}
	var res protocol.ConfigureResult
	if p.File != nil {
		for _, d := range p.File.Directives {
			if d.Key != enabledDirective {
				continue
			}
			switch v := strings.TrimSpace(d.Value); v {
			case "true", "false":
				cfg.Enabled = v == "true"
			default:
				res.Errors = append(res.Errors, protocol.Diagnostic{
					Message: fmt.Sprintf("%s: gazelle:%s: expected true or false, got %q", p.File.Path, enabledDirective, d.Value),
				})
			}
		}
	}
	res.Config, err = json.Marshal(cfg)
	return res, err
}

// Generate implements [server.Generator].
func (*Plugin) Generate(ctx context.Context, p protocol.GenerateParams) (protocol.GenerateResult, error) {
	cfg, err := parseConfig(p.Config)
	if err != nil {
		return protocol.GenerateResult{}, err
	}
	var res protocol.GenerateResult
	if !cfg.Enabled {
		return res, nil
	}
	var scripts []string
	for _, f := range p.RegularFiles {
		if strings.HasSuffix(f, ".sh") {
			scripts = append(scripts, f)
		}
	}
	slices.Sort(scripts)

	generated := make(map[string]bool)
	for _, script := range scripts {
		content, err := os.ReadFile(filepath.Join(p.Dir, filepath.FromSlash(script)))
		if err != nil {
			res.Errors = append(res.Errors, protocol.Diagnostic{Message: err.Error()})
			continue
		}
		name := strings.TrimSuffix(path.Base(script), ".sh")
		k := "sh_library"
		if strings.HasSuffix(name, "_test") {
			k = "sh_test"
		} else if strings.HasPrefix(string(content), "#!") {
			k = "sh_binary"
		}
		r := protocol.Rule{
			Kind:  k,
			Name:  name,
			Attrs: map[string]any{"srcs": []string{script}},
		}
		if k == "sh_library" {
			r.Attrs["visibility"] = []string{"//visibility:public"}
		}
		imports, err := json.Marshal(parseSources(string(content), p.Rel))
		if err != nil {
			return protocol.GenerateResult{}, err
		}
		res.Gen = append(res.Gen, r)
		res.Imports = append(res.Imports, imports)
		generated[name] = true
	}

	// Rules for scripts that no longer exist are reported as empty, so Gazelle
	// can delete them.
	if p.File != nil {
		for _, r := range p.File.Rules {
			if isShKind(r.Kind) && r.Name != "" && !generated[r.Name] {
				res.Empty = append(res.Empty, protocol.Rule{Kind: r.Kind, Name: r.Name})
			}
		}
	}
	return res, nil
}

// parseSources returns the repository-relative paths of files sourced by a
// script in the directory rel.
func parseSources(content, rel string) []string {
	var paths []string
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || (fields[0] != "source" && fields[0] != ".") {
			continue
		}
		p := strings.Trim(fields[1], `"'`)
		if p == "" || strings.Contains(p, "$") || strings.HasPrefix(p, "/") {
			continue
		}
		if strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") {
			p = path.Join(rel, p)
		}
		paths = append(paths, p)
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)
	if paths == nil {
		paths = []string{}
	}
	return paths
}

// Imports implements [server.Indexer]. Libraries are indexed by the
// repository-relative paths of their sources.
func (*Plugin) Imports(ctx context.Context, p protocol.ImportsParams) (protocol.ImportsResult, error) {
	switch p.Rule.Kind {
	case "sh_library":
		var res protocol.ImportsResult
		srcs, _ := p.Rule.Attrs["srcs"].([]any)
		for _, src := range srcs {
			if s, ok := src.(string); ok {
				res.Imports = append(res.Imports, protocol.ImportSpec{Lang: langName, Imp: path.Join(p.Rel, s)})
			}
		}
		return res, nil
	case "sh_binary", "sh_test":
		return protocol.ImportsResult{NotImportable: true}, nil
	default:
		return protocol.ImportsResult{}, nil
	}
}

// Resolve implements [server.Resolver]. It looks up each sourced file in
// Gazelle's rule index and sets deps.
func (*Plugin) Resolve(ctx context.Context, p protocol.ResolveParams) (protocol.ResolveResult, error) {
	var imports []string
	if len(p.Imports) > 0 {
		if err := json.Unmarshal(p.Imports, &imports); err != nil {
			return protocol.ResolveResult{}, err
		}
	}
	var res protocol.ResolveResult
	var deps []string
	for _, imp := range imports {
		found, err := server.IndexFind(ctx, protocol.IndexFindParams{
			Import: protocol.ImportSpec{Lang: langName, Imp: imp},
		})
		if err != nil {
			return protocol.ResolveResult{}, err
		}
		self := false
		var matched []string
		for _, m := range found.Results {
			if m.SelfImport {
				self = true
				continue
			}
			matched = append(matched, m.RelLabel)
		}
		if len(matched) == 0 && !self {
			res.Errors = append(res.Errors, protocol.Diagnostic{
				Message:  fmt.Sprintf("%s: no rule provides sourced file %s", p.From, imp),
				Severity: "warning",
			})
		}
		deps = append(deps, matched...)
	}
	if len(deps) > 0 {
		slices.Sort(deps)
		res.Attrs = map[string]any{"deps": slices.Compact(deps)}
	}
	return res, nil
}
