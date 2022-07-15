/* Copyright 2016 The Bazel Authors. All rights reserved.

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

// Command gazelle is a BUILD file generator for Go projects.
// See "gazelle --help" for more details.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/bazelbuild/bazel-gazelle/runner"
)

func main() {
	log.SetPrefix("gazelle: ")
	log.SetFlags(0) // don't print timestamps

	wd, err := runner.GetDefaultWorkspaceDirectory()
	if err != nil {
		log.Fatal(err)
	}

	if err := runner.Run(languages, wd, os.Args[1:]); err != nil && err != flag.ErrHelp {
		if err == runner.ErrDiff {
			os.Exit(1)
		} else {
			log.Fatal(err)
		}
	}
}
