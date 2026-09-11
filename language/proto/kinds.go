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

package proto

import (
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

var protoLibraryKind = rule.KindInfo{
	Name:          "proto_library",
	LoadedFrom:    label.New(protobufModuleName, "bazel", "proto_library.bzl"),
	MatchAttrs:    []string{"srcs"},
	NonEmptyAttrs: map[string]bool{"srcs": true},
	MergeableAttrs: map[string]bool{
		"srcs":                true,
		"import_prefix":       true,
		"strip_import_prefix": true,
	},
	ResolveAttrs: map[string]bool{"deps": true},
}

func (*protoLang) Kinds() []rule.KindInfo {
	return []rule.KindInfo{protoLibraryKind}
}
