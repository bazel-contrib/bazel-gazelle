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

// Package wspace provides functions to locate and modify a bazel WORKSPACE file.
package wspace

import (
	"os"
	"path/filepath"
	"strings"
)

var workspaceFiles = []string{"WORKSPACE.bazel", "WORKSPACE"}

// FindWorkspaceInRoot returns a path to a file in the provided root directory,
// either to an existing WORKSPACE or WORKSPACE.bazel file, or to root/WORKSPACE
// by default. Note that this function does NOT recursively check parent directories
func FindWorkspaceInRoot(root string) string {
	for _, workspaceFile := range workspaceFiles {
		path := filepath.Join(root, workspaceFile)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(root, "WORKSPACE")
}

// FindWorkspaceFile searches from the given dir and up for the WORKSPACE file
// returning the absolute filepath of the given file
func FindWorkspaceFile(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	for {
		for _, workspaceFile := range workspaceFiles {
			filepath := filepath.Join(dir, workspaceFile)
			_, err = os.Stat(filepath)
			if err == nil {
				return filepath, nil
			}
			if !os.IsNotExist(err) {
				return "", err
			}
		}
		if strings.HasSuffix(dir, string(os.PathSeparator)) { // stop at root dir
			return "", os.ErrNotExist
		}
		dir = filepath.Dir(dir)
	}
}

// FindRoot searches from the given dir and up for a directory containing a WORKSPACE file
// returning the directory containing it, or an error if none found in the tree.
func FindRoot(dir string) (string, error) {
	file, err := FindWorkspaceFile(dir)
	if err != nil {
		return "", err
	}
	return filepath.Dir(file), nil
}
