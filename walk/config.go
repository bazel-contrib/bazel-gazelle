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

package walk

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
	"github.com/bmatcuk/doublestar/v4"

	gzflag "github.com/bazelbuild/bazel-gazelle/flag"
)

// generationModeType represents one of the generation modes.
type generationModeType string

// Generation modes
const (
	// Update: update and maintain existing BUILD files
	generationModeUpdate generationModeType = "update_only"

	// Create: create new and update existing BUILD files
	generationModeCreate generationModeType = "create_and_update"
)

// TODO(#472): store location information to validate each exclude. They
// may be set in one directory and used in another. Excludes work on
// declared generated files, so we can't just stat.

type walkConfig struct {
	updateOnly bool
	excludes   []string
	ignore     bool
	follow     []string
}

const walkName = "_walk"

func getWalkConfig(c *config.Config) *walkConfig {
	return c.Exts[walkName].(*walkConfig)
}

func (wc *walkConfig) isExcluded(p string) bool {
	return matchAnyGlob(wc.excludes, p)
}

func (wc *walkConfig) shouldFollow(p string) bool {
	return matchAnyGlob(wc.follow, p)
}

var _ config.Configurer = (*Configurer)(nil)

type Configurer struct{}

func (*Configurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	wc := &walkConfig{}
	c.Exts[walkName] = wc
	fs.Var(&gzflag.MultiFlag{Values: &wc.excludes}, "exclude", "pattern that should be ignored (may be repeated)")
}

func (*Configurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error { return nil }

func (*Configurer) KnownDirectives() []string {
	return []string{"generation_mode", "exclude", "follow", "ignore"}
}

func (cr *Configurer) Configure(c *config.Config, rel string, f *rule.File) {
	wc := getWalkConfig(c)
	wcCopy := &walkConfig{}
	*wcCopy = *wc
	wcCopy.ignore = false

	if f != nil {
		for _, d := range f.Directives {
			switch d.Key {
			case "generation_mode":
				switch generationModeType(strings.TrimSpace(d.Value)) {
				case generationModeUpdate:
					wcCopy.updateOnly = true
				case generationModeCreate:
					wcCopy.updateOnly = false
				default:
					log.Fatalf("unknown generation_mode %q in //%s", d.Value, f.Pkg)
					continue
				}
			case "exclude":
				if err := checkPathMatchPattern(path.Join(rel, d.Value)); err != nil {
					log.Printf("the exclusion pattern is not valid %q: %s", path.Join(rel, d.Value), err)
					continue
				}
				wcCopy.excludes = append(wcCopy.excludes, path.Join(rel, d.Value))
			case "follow":
				if err := checkPathMatchPattern(path.Join(rel, d.Value)); err != nil {
					log.Printf("the follow pattern is not valid %q: %s", path.Join(rel, d.Value), err)
					continue
				}
				wcCopy.follow = append(wcCopy.follow, path.Join(rel, d.Value))
			case "ignore":
				if d.Value != "" {
					log.Printf("the ignore directive does not take any arguments. Did you mean to use gazelle:exclude instead? in //%s '# gazelle:ignore %s'", f.Pkg, d.Value)
				}
				wcCopy.ignore = true
			}
		}
	}

	c.Exts[walkName] = wcCopy
}

type isIgnoredFunc = func(string) bool

var nothingIgnored isIgnoredFunc = func(string) bool { return false }

func loadBazelIgnore(repoRoot string) (isIgnoredFunc, error) {
	ignorePath := path.Join(repoRoot, ".bazelignore")
	file, err := os.Open(ignorePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nothingIgnored, nil
	}
	if err != nil {
		return nothingIgnored, fmt.Errorf(".bazelignore exists but couldn't be read: %v", err)
	}
	defer file.Close()

	excludes := make(map[string]struct{})

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ignore := strings.TrimSpace(scanner.Text())
		if ignore == "" || string(ignore[0]) == "#" {
			continue
		}
		// Bazel ignore paths are always relative to repo root.
		// Glob patterns are not supported.
		if strings.ContainsAny(ignore, "*?[") {
			log.Printf("the .bazelignore exclusion pattern must not be a glob %s", ignore)
			continue
		}

		// Clean the path to remove any extra '.', './' etc otherwise
		// the exclude matching won't work correctly.
		ignore = path.Clean(ignore)

		excludes[ignore] = struct{}{}
	}

	isBazelIgnored := func(p string) bool {
		_, ok := excludes[p]
		return ok
	}

	return isBazelIgnored, nil
}

func loadRepoDirectoryIgnore(repoRoot string) (isIgnoredFunc, error) {
	repoFilePath := path.Join(repoRoot, "REPO.bazel")
	repoFileContent, err := os.ReadFile(repoFilePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nothingIgnored, nil
	}
	if err != nil {
		return nothingIgnored, fmt.Errorf("REPO.bazel exists but couldn't be read: %v", err)
	}

	ast, err := bzl.Parse(repoRoot, repoFileContent)
	if err != nil {
		return nothingIgnored, fmt.Errorf("failed to parse REPO.bazel: %v", err)
	}

	var ignoreDirectories []string

	// Search for ignore_directories([...ignore strings...])
	for _, expr := range ast.Stmt {
		if call, isCall := expr.(*bzl.CallExpr); isCall {
			if inv, isIdentCall := call.X.(*bzl.Ident); isIdentCall && inv.Name == "ignore_directories" {
				if len(call.List) != 1 {
					return nothingIgnored, fmt.Errorf("REPO.bazel ignore_directories() expects one argument")
				}

				list, isList := call.List[0].(*bzl.ListExpr)
				if !isList {
					return nothingIgnored, fmt.Errorf("REPO.bazel ignore_directories() unexpected argument type: %T", call.List[0])
				}

				for _, item := range list.List {
					if strExpr, isStr := item.(*bzl.StringExpr); isStr {
						if err := checkPathMatchPattern(strExpr.Value); err != nil {
							log.Printf("the ignore_directories() pattern %q is not valid: %s", strExpr.Value, err)
							continue
						}

						ignoreDirectories = append(ignoreDirectories, strExpr.Value)
					}
				}

				// Only a single ignore_directories() is supported in REPO.bazel and searching can stop.
				break
			}
		}
	}

	if len(ignoreDirectories) == 0 {
		return nothingIgnored, nil
	}

	isRepoIgnored := func(p string) bool {
		for _, ignore := range ignoreDirectories {
			if doublestar.MatchUnvalidated(ignore, p) {
				return true
			}
		}
		return false
	}

	return isRepoIgnored, nil
}

func checkPathMatchPattern(pattern string) error {
	_, err := doublestar.Match(pattern, "x")
	return err
}

func matchAnyGlob(patterns []string, path string) bool {
	for _, x := range patterns {
		if doublestar.MatchUnvalidated(x, path) {
			return true
		}
	}
	return false
}
