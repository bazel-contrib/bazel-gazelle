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

// Package walk provides customizable functionality for visiting each
// subdirectory in a directory tree.
package walk

import (
	"github.com/bazel-contrib/bazel-gazelle/v2/compat"
	v2config "github.com/bazel-contrib/bazel-gazelle/v2/config"
	v2 "github.com/bazel-contrib/bazel-gazelle/v2/walk"
	"github.com/bazelbuild/bazel-gazelle/config"
)

// Mode determines which directories Walk visits and which directories
// should be updated.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.Mode instead.
type Mode = v2.Mode

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.VisitAllUpdateSubdirsMode instead.
const VisitAllUpdateSubdirsMode = v2.VisitAllUpdateSubdirsMode

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.VisitAllUpdateDirsMode instead.
const VisitAllUpdateDirsMode = v2.VisitAllUpdateDirsMode

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.UpdateDirsMode instead.
const UpdateDirsMode = v2.UpdateDirsMode

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.UpdateSubdirsMode instead.
const UpdateSubdirsMode = v2.UpdateSubdirsMode

// WalkFunc is a callback called by Walk in each visited directory.
//
// dir is the absolute file system path to the directory being visited.
//
// rel is the relative slash-separated path to the directory from the
// repository root. Will be "" for the repository root directory itself.
//
// c is the configuration for the current directory. This may have been
// modified by directives in the directory's build file.
//
// update is true when the build file may be updated.
//
// f is the existing build file in the directory. Will be nil if there
// was no file.
//
// subdirs is a list of base names of subdirectories within dir, not
// including excluded files.
//
// regularFiles is a list of base names of regular files within dir, not
// including excluded files or symlinks.
//
// genFiles is a list of names of generated files, found by reading
// "out" and "outs" attributes of rules in f.
//
// DEPRECATED: Use Walk2Func with Walk2 instead.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.WalkFunc instead.
type WalkFunc = v2.WalkFunc

// Walk traverses the directory tree rooted at c.RepoRoot. Walk visits
// subdirectories in depth-first post-order.
//
// When Walk visits a directory, it lists the files and subdirectories within
// that directory. If a build file is present, Walk reads the build file and
// applies any directives to the configuration (a copy of the parent directory's
// configuration is made, and the copy is modified). After visiting
// subdirectories, the callback wf may be called, depending on the mode.
//
// c is the root configuration to start with. This includes changes made by
// command line flags, but not by the root build file. This configuration
// should not be modified.
//
// cexts is a list of configuration extensions. When visiting a directory,
// before visiting subdirectories, Walk makes a copy of the parent configuration
// and Configure for each extension on the copy. If Walk sees a directive
// that is not listed in KnownDirectives of any extension, an error will
// be logged.
//
// dirs is a list of absolute, canonical file system paths of directories
// to visit.
//
// mode determines whether subdirectories of dirs should be visited recursively,
// when the wf callback should be called, and when the "update" argument
// to the wf callback should be set.
//
// wf is a function that may be called in each directory.
//
// DEPRECATED: Use Walk2 instead.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.Walk instead.
func Walk(c *config.Config, cexts []config.Configurer, dirs []string, mode Mode, wf WalkFunc) {
	v2cexts := make([]v2config.Configurer, len(cexts))
	for i, cext := range cexts {
		v2cexts[i] = compat.MustConfigurerV2(cext)
	}
	v2.Walk(c, v2cexts, dirs, mode, wf)
}

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.Walk2Func instead.
type Walk2Func = v2.Walk2Func

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.Walk2FuncArgs instead.
type Walk2FuncArgs = v2.Walk2FuncArgs

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.Walk2FuncResult instead.
type Walk2FuncResult = v2.Walk2FuncResult

// Walk2 traverses a limited part of the directory tree rooted at c.RepoRoot
// and calls the function wf in each visited directory.
//
// The dirs and mode parameters determine which directories Walk2 visits.
// Walk2 calls wf in each directory in dirs with the Walk2FuncArgs.Update
// flag set to true. This indicates Gazelle should update build files in that
// directory. Depending on the mode flag, Walk2 may additionally visit
// subdirectories or all directories in the repo, possibly with the Update
// flag set.
//
// Some directives like "# gazelle:exclude" and files like .bazelignore
// control the traversal, excluding certain files and directories.
//
// The traversal is done in post-order, but configuration directives are always
// applied from build files in parent directories first. Concretely, this means
// that language.Configurer.Configure is called on each extension in cexts in a
// directory *before* visiting its subdirectories; wf is called in a directory
// *after* its subdirectories.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.Walk2 instead.
func Walk2(c *config.Config, cexts []config.Configurer, dirs []string, mode Mode, wf Walk2Func) error {
	v2cexts := make([]v2config.Configurer, len(cexts))
	for i, cext := range cexts {
		v2cexts[i] = compat.MustConfigurerV2(cext)
	}
	return v2.Walk2(c, v2cexts, dirs, mode, wf)
}
