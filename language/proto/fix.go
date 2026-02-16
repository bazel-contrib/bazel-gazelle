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

const deprecatedRulesFile = "@rules_proto//proto:defs.bzl"

// Maps all old symbols from:
// https://github.com/bazelbuild/rules_proto/blob/main/proto/defs.bzl
// to their new file locations in the directory:
// https://github.com/protocolbuffers/protobuf/tree/main/bazel
var newBzlFileOfSymbol = map[string]label.Label{
	"proto_library":        label.New("", "bazel", "proto_library.bzl"),
	"proto_descriptor_set": label.New("", "bazel", "proto_descriptor_set.bzl"),
	"proto_lang_toolchain": label.New("", "bazel/toolchains", "proto_lang_toolchain.bzl"),
	"proto_toolchain":      label.New("", "bazel/toolchains", "proto_toolchain.bzl"),
	"ProtoInfo":            label.New("", "bazel/common", "proto_info.bzl"),
	"proto_common":         label.New("", "bazel/common", "proto_common.bzl"),
}

func (*protoLang) Fix(c *config.Config, f *rule.File) {
	protoModule := c.ModuleToApparentName("protobuf")
	if protoModule == "" {
		// No protobuf dependency in MODULE.bazel, nothing to fix
		return
	}

	var rulesProtoLoads []*rule.Load
	for _, l := range f.Loads {
		if l.Name() == deprecatedRulesFile {
			rulesProtoLoads = append(rulesProtoLoads, l)
		}
	}

	if len(rulesProtoLoads) == 0 {
		return
	}

	if !c.ShouldFix {
		log.Printf("%s: %s is deprecated. Run 'gazelle fix' to replace with new load statement.", f.Path, deprecatedRulesFile)
		return
	}

	for _, l := range rulesProtoLoads {
		hasUnknownSymbol := false
		for _, sym := range l.Symbols() {
			if newBzlFile, ok := newBzlFileOfSymbol[sym]; ok {
				// Match the apparent name from MODULE.bazel
				newBzlFile.Repo = protoModule

				// Add the new load statement nearby the old one
				newLoad := rule.NewLoad(newBzlFile.String())
				newLoad.Add(sym)
				newLoad.Insert(f, l.Index())
			} else {
				hasUnknownSymbol = true
				log.Printf("%s: unknown symbol %q loaded from %s", f.Path, sym, deprecatedRulesFile)
			}
		}

		if !hasUnknownSymbol {
			l.Delete()
		}
	}
}
