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

// sh_plugin is a Gazelle plugin for shell scripts. Run Gazelle with
// -plugin=path/to/sh_plugin to use it.
package main

import (
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/examples/sh"
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/server"
)

func main() {
	server.Main(sh.New())
}
