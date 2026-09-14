/* Copyright 2018 The Bazel Authors. All rights reserved.

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

package golang

import (
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

var (
	rulesGoGoDefBzl    = label.New("rules_go", "go", "def.bzl")
	rulesGoProtoDefBzl = label.New("rules_go", "proto", "def.bzl")
	gazelleDepsBzl     = label.New("gazelle", "", "deps.bzl")
)

var goKinds = []rule.KindInfo{{
	Name:       "cgo_library",
	LoadedFrom: rulesGoGoDefBzl,
}, {
	Name:       "go_binary",
	LoadedFrom: rulesGoGoDefBzl,
	MatchAny:   true,
	NonEmptyAttrs: map[string]bool{
		"deps":  true,
		"embed": true,
		"srcs":  true,
	},
	SubstituteAttrs: map[string]bool{"embed": true},
	MergeableAttrs: map[string]bool{
		"cgo":         true,
		"clinkopts":   true,
		"cppopts":     true,
		"copts":       true,
		"cxxopts":     true,
		"embed":       true,
		"embedsrcs":   true,
		"gc_goopts":   true,
		"gc_linkopts": true,
		"pgoprofile":  true,
		"srcs":        true,
	},
	ResolveAttrs: map[string]bool{"deps": true},
}, {
	Name:       "go_library",
	LoadedFrom: rulesGoGoDefBzl,
	MatchAttrs: []string{"importpath"},
	NonEmptyAttrs: map[string]bool{
		"deps":  true,
		"embed": true,
		"srcs":  true,
	},
	SubstituteAttrs: map[string]bool{
		"embed": true,
	},
	MergeableAttrs: map[string]bool{
		"cgo":        true,
		"clinkopts":  true,
		"cppopts":    true,
		"copts":      true,
		"cxxopts":    true,
		"embed":      true,
		"embedsrcs":  true,
		"gc_goopts":  true,
		"importmap":  true,
		"importpath": true,
		"srcs":       true,
	},
	ResolveAttrs: map[string]bool{"deps": true},
}, {
	Name:       "go_proto_library",
	LoadedFrom: rulesGoProtoDefBzl,
	MatchAttrs: []string{"importpath"},
	NonEmptyAttrs: map[string]bool{
		"deps":   true,
		"embed":  true,
		"proto":  true,
		"protos": true,
		"srcs":   true,
	},
	SubstituteAttrs: map[string]bool{"proto": true, "protos": true},
	MergeableAttrs: map[string]bool{
		"srcs":       true,
		"importpath": true,
		"importmap":  true,
		"cgo":        true,
		"clinkopts":  true,
		"cppopts":    true,
		"copts":      true,
		"cxxopts":    true,
		"embed":      true,
		"proto":      true,
		"protos":     true,
		"compilers":  true,
	},
	ResolveAttrs: map[string]bool{"deps": true},
}, {
	Name:       "go_grpc_library",
	LoadedFrom: rulesGoProtoDefBzl,
}, {
	Name:       "go_repository",
	LoadedFrom: gazelleDepsBzl,
	MatchAttrs: []string{"importpath"},
	NonEmptyAttrs: map[string]bool{
		"importpath": true,
	},
	MergeableAttrs: map[string]bool{
		"commit":       true,
		"build_tags":   true,
		"importpath":   true,
		"remote":       true,
		"replace":      true,
		"sha256":       true,
		"strip_prefix": true,
		"sum":          true,
		"tag":          true,
		"type":         true,
		"urls":         true,
		"vcs":          true,
		"version":      true,
	},
}, {
	Name:       "go_test",
	LoadedFrom: rulesGoGoDefBzl,
	NonEmptyAttrs: map[string]bool{
		"deps":  true,
		"embed": true,
		"srcs":  true,
	},
	MergeableAttrs: map[string]bool{
		"cgo":         true,
		"clinkopts":   true,
		"cppopts":     true,
		"copts":       true,
		"cxxopts":     true,
		"embed":       true,
		"embedsrcs":   true,
		"gc_goopts":   true,
		"gc_linkopts": true,
		"srcs":        true,
	},
	ResolveAttrs: map[string]bool{"deps": true},
}, {
	// HACK(#834): remove when bazelbuild/rules_go#2374 is resolved.
	Name:       "go_tool_library",
	LoadedFrom: rulesGoGoDefBzl,
	MatchAttrs: []string{"importpath"},
	NonEmptyAttrs: map[string]bool{
		"deps":  true,
		"embed": true,
		"srcs":  true,
	},
	SubstituteAttrs: map[string]bool{
		"embed": true,
	},
	MergeableAttrs: map[string]bool{
		"cgo":        true,
		"clinkopts":  true,
		"cppopts":    true,
		"copts":      true,
		"cxxopts":    true,
		"embed":      true,
		"importmap":  true,
		"importpath": true,
		"srcs":       true,
	},
	ResolveAttrs: map[string]bool{"deps": true},
}}

var goKindsMap map[string]rule.KindInfo

func init() {
	goKindsMap = make(map[string]rule.KindInfo, len(goKinds))
	for _, k := range goKinds {
		goKindsMap[k.Name] = k
	}
}

func (*goLang) Kinds() []rule.KindInfo {
	return goKinds
}
