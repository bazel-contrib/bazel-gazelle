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
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func (*protoLang) Fix(c *config.Config, f *rule.File) {
	// Check if the module depends on protobuf
	protoModule := c.ModuleToApparentName("protobuf")
	if protoModule == "" {
		// No protobuf dependency in MODULE.bazel, nothing to fix
		return
	}

	// Find loads from @rules_proto//proto:defs.bzl
	var rulesProtoLoads []*rule.Load
	for _, l := range f.Loads {
		if l.Name() == "@rules_proto//proto:defs.bzl" {
			rulesProtoLoads = append(rulesProtoLoads, l)
		}
	}

	if len(rulesProtoLoads) == 0 {
		return
	}

	if !c.ShouldFix {
		log.Printf("%s: @rules_proto//proto:defs.bzl is deprecated. Run 'gazelle fix' to replace with new load statement.", f.Path)
		return
	}

	// Delete the old load statements. The merger will restore them
	// with the correct module name from ApparentLoads.
	for _, l := range rulesProtoLoads {
		l.Delete()
	}
}
