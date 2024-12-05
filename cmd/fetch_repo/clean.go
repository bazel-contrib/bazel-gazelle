/* Copyright 2024 The Bazel Authors. All rights reserved.

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

package main

import (
	"io/fs"
	"os"
	"path/filepath"
)

func cleanBuildFiles(path string) error {
	filenamesToClean := []string{
		"BUILD",
		"BUILD.bazel",
		"MODULE.bazel",
		"MODULE.bazel.lock",
		"WORKSPACE",
		"WORKSPACE.bazel",
		"WORKSPACE.bzlmod",
	}
	return filepath.Walk(*dest, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		for _, filename := range filenamesToClean {
			if info.Name() == filename {
				return os.Remove(path)
			}
		}
		return nil
	})
}
