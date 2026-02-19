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
	"log"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const (
	protobufModuleName    = "protobuf"
	protobufWorkspaceName = "com_google_protobuf"
	rulesProtoModuleName  = "rules_proto"
)

// Returns the file name of of a deprecated load statement from @rules_proto.
// Used to identify which load statements to fix.
func deprecatedFileLabel(moduleToApparentName func(string) string) label.Label {
	repoName := moduleToApparentName(rulesProtoModuleName)
	if repoName == "" {
		repoName = rulesProtoModuleName
	}

	return label.New(repoName, "proto", "defs.bzl")
}

// Maps all old symbols from:
// https://github.com/bazelbuild/rules_proto/blob/main/proto/defs.bzl
// to their new file locations in the directory:
// https://github.com/protocolbuffers/protobuf/tree/main/bazel
func symbolToFileLabel(moduleToApparentName func(string) string, sym string) label.Label {
	repoName := moduleToApparentName(protobufModuleName)
	if repoName == "" {
		// Support legacy WORKSPACE files and fallback to the old repo name
		repoName = protobufWorkspaceName
	}

	switch sym {
	case "proto_library":
		return label.New(repoName, "bazel", "proto_library.bzl")
	case "proto_descriptor_set":
		return label.New(repoName, "bazel", "proto_descriptor_set.bzl")
	case "proto_lang_toolchain":
		return label.New(repoName, "bazel/toolchains", "proto_lang_toolchain.bzl")
	case "proto_toolchain":
		return label.New(repoName, "bazel/toolchains", "proto_toolchain.bzl")
	case "ProtoInfo":
		return label.New(repoName, "bazel/common", "proto_info.bzl")
	case "proto_common":
		return label.New(repoName, "bazel/common", "proto_common.bzl")
	default:
		return label.NoLabel
	}
}

func hasProtobufModuleDependency(c *config.Config) bool {
	return c.ModuleToApparentName(protobufModuleName) != ""
}

func (*protoLang) Fix(c *config.Config, f *rule.File) {
	if !hasProtobufModuleDependency(c) {
		return
	}

	// Collect deprecated Load statements
	deprecatedFile := deprecatedFileLabel(c.ModuleToApparentName).String()
	deprecatedLoads := make([]*rule.Load, 0, len(f.Loads))
	for _, l := range f.Loads {
		if l.Name() == deprecatedFile {
			deprecatedLoads = append(deprecatedLoads, l)
		}
	}

	if len(deprecatedLoads) == 0 {
		return
	}

	// Replace the deprecated load statements with the new load statements for
	// each symbol
	for _, l := range deprecatedLoads {
		l.Delete()
		for _, sym := range l.Symbols() {
			if newFileLabel := symbolToFileLabel(c.ModuleToApparentName, sym); newFileLabel != label.NoLabel {
				newLoad := rule.NewLoad(newFileLabel.String())
				newLoad.Add(sym)
				newLoad.Insert(f, l.Index())
			} else {
				log.Printf("%s: unknown symbol %q loaded from %s", f.Path, sym, deprecatedFile)
			}
		}
	}
}
