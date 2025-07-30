/* Copyright 2023 The Bazel Authors. All rights reserved.

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

// Package module provides functions to read information out of MODULE.bazel files.

package module

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/buildtools/build"
)

// ExtractModuleToApparentNameMapping collects the mapping of module names (e.g. "rules_go") to
// user-configured apparent names (e.g. "my_rules_go") from the repos MODULE.bazel, if it exists.
// See https://bazel.build/external/module#repository_names_and_strict_deps for more information on
// apparent names.
func ExtractModuleToApparentNameMapping(repoRoot string) (func(string) string, error) {
	moduleToApparentName, err := collectApparentNames(repoRoot, "MODULE.bazel")
	if err != nil {
		return nil, err
	}

	return func(moduleName string) string {
		return moduleToApparentName[moduleName]
	}, nil
}

// ExtractModuleName collects name of the module from the MODULE.bazel file, if it exists.
// Returns empty string if MODULE.bazel does not exist or does not define explicit name
func ExtractModuleName(repoRoot string) (string, error) {
	f, err := parseModuleSegment(repoRoot, "MODULE.bazel")
	if err != nil {
		// If there is no MODULE.bazel file, return an name by no error
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, dep := range f.Rules("") {
		if dep.Kind() == "module" {
			return dep.Name(), nil
		}
	}
	return "", nil
}

func parseModuleSegment(repoRoot, relPath string) (*build.File, error) {
	path := filepath.Join(repoRoot, relPath)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return build.ParseModule(path, bytes)
}

// Collects the mapping of module names (e.g. "rules_go") to user-configured apparent names (e.g.
// "my_rules_go"). See https://bazel.build/external/module#repository_names_and_strict_deps for more
// information on apparent names.
func collectApparentNames(repoRoot, relPath string) (map[string]string, error) {
	apparentNames := make(map[string]string)
	seenFiles := make(map[string]struct{})
	filesToProcess := []string{relPath}

	for len(filesToProcess) > 0 {
		f := filesToProcess[0]
		filesToProcess = filesToProcess[1:]
		if _, seen := seenFiles[f]; seen {
			continue
		}
		seenFiles[f] = struct{}{}
		bf, err := parseModuleSegment(repoRoot, f)
		if err != nil {
			if f == "MODULE.bazel" && os.IsNotExist(err) {
				// If there is no MODULE.bazel file, return an empty map but no error.
				// Languages will know to fall back to the WORKSPACE names of repos.
				return nil, nil
			}
			return nil, err
		}
		names, includeLabels := collectApparentNamesAndIncludes(bf)
		for name, apparentName := range names {
			apparentNames[name] = apparentName
		}
		for _, includeLabel := range includeLabels {
			l, err := label.Parse(includeLabel)
			if err != nil {
				return nil, fmt.Errorf("failed to parse include label %q: %v", includeLabel, err)
			}
			p := filepath.Join(filepath.FromSlash(l.Pkg), filepath.FromSlash(l.Name))
			filesToProcess = append(filesToProcess, p)
		}
	}

	return apparentNames, nil
}

func collectApparentNamesAndIncludes(f *build.File) (map[string]string, []string) {
	apparentNames := make(map[string]string)
	var includeLabels []string

	for _, dep := range f.Rules("") {
		if dep.ExplicitName() == "" {
			if ident, ok := dep.Call.X.(*build.Ident); !ok || ident.Name != "include" {
				continue
			}
			if len(dep.Call.List) != 1 {
				continue
			}
			if str, ok := dep.Call.List[0].(*build.StringExpr); ok {
				includeLabels = append(includeLabels, str.Value)
			}
			continue
		}
		if dep.Kind() != "module" && dep.Kind() != "bazel_dep" {
			continue
		}
		// We support module in addition to bazel_dep to handle language repos that use Gazelle to
		// manage their own BUILD files.
		if name := dep.AttrString("name"); name != "" {
			if repoName := dep.AttrString("repo_name"); repoName != "" {
				apparentNames[name] = repoName
			} else {
				apparentNames[name] = name
			}
		}
	}

	return apparentNames, includeLabels
}
