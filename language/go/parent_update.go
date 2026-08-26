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

package golang

import (
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/walk"
)

type goDirState struct {
	goFileInfos           map[string]fileInfo
	updatesForDescendants []string
}

// configureDirState records parsed Go files and non-local inputs while
// directories are configured in parent-first order.
func (gl *goLang) configureDirState(c *config.Config, gc *goConfig, rel string, f *rule.File) {
	if rel == "" || gl.dirStates == nil {
		gl.dirStates = make(map[string]goDirState)
	}

	var state goDirState

	// A BUILD file is a package boundary, so inherited relationships don't
	// propagate past it. The boundary itself may still request those updates.
	if f == nil {
		state.updatesForDescendants = gl.ancestorUpdates(rel)
	}

	dirInfo, err := walk.GetDirInfo(rel)
	if err != nil {
		gl.dirStates[rel] = state
		return
	}
	var hasEmbed bool
	state.goFileInfos, hasEmbed = cacheGoFiles(c, gc, rel, dirInfo.RegularFiles)
	if hasEmbed {
		state.updatesForDescendants = append(state.updatesForDescendants, rel)
	}
	gl.dirStates[rel] = state
}

// ancestorUpdates returns ancestor directories whose generated rules may
// depend on files in rel.
func (gl *goLang) ancestorUpdates(rel string) []string {
	if rel == "" {
		return nil
	}

	parent := parentRel(rel)
	parentState := gl.dirStates[parent]
	updates := slices.Clone(parentState.updatesForDescendants)
	// Go rules may include a non-empty testdata subtree. Request the parent
	// without duplicating Go package selection during configuration.
	if path.Base(rel) == "testdata" && !slices.Contains(updates, parent) {
		updates = append(updates, parent)
	}
	return updates
}

func cacheGoFiles(c *config.Config, gc *goConfig, rel string, regularFiles []string) (infos map[string]fileInfo, hasEmbed bool) {
	dir := filepath.Join(c.RepoRoot, filepath.FromSlash(rel))
	srcdir := goSrcDir(c, gc, rel)
	for _, name := range regularFiles {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		info := goFileInfo(filepath.Join(dir, name), srcdir)
		if infos == nil {
			infos = make(map[string]fileInfo)
		}
		infos[name] = info
		if info.ext == goExt && info.packageName != "documentation" {
			// Any go:embed directive may establish the relationship. Generation
			// decides later whether the file is selected for the current platform.
			hasEmbed = hasEmbed || len(info.embeds) > 0
		}
	}
	return infos, hasEmbed
}

func goSrcDir(c *config.Config, gc *goConfig, rel string) string {
	if !gc.goRepositoryMode {
		return rel
	}

	// cgo opts such as '-L${SRCDIR}/libs' should become
	// '-Lexternal/my_repo~/libs' in an external repo. Obtain the path from the
	// repository root to support both repository layouts.
	slashPath := filepath.ToSlash(c.RepoRoot)
	segments := strings.Split(slashPath, "/")
	repoName := segments[len(segments)-1]
	if segments[len(segments)-2] == "external" {
		return path.Join("external", repoName, rel)
	}
	return path.Join("..", repoName, rel)
}

func parentRel(rel string) string {
	parent := path.Dir(rel)
	if parent == "." {
		return ""
	}
	return parent
}
