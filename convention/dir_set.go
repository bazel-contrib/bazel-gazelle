// Copyright 2024 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package convention

import (
	"path/filepath"
	"strings"
)

// DirSet keeps track of a set of relative paths
type DirSet struct {
	dirs    map[string]bool
	hasRoot bool
}

// NewDirSet initialize a new DirSet
func NewDirSet(dirs []string) DirSet {
	ds := DirSet{dirs: make(map[string]bool)}
	for _, d := range dirs {
		ds.dirs[strings.TrimSpace(d)] = true
		if d == "" || d == "." {
			ds.hasRoot = true
		}
	}
	return ds
}

// HasSubDir decides whether a relative path is one of the directories or in one of their subdirectories
func (ds DirSet) HasSubDir(dir string) bool {
	if ds.hasRoot {
		return true
	}
	for ; dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		if ds.hasDir(dir) {
			return true
		}
	}
	return false
}

func (ds DirSet) hasDir(dir string) bool {
	_, ok := ds.dirs[dir]
	return ok
}
