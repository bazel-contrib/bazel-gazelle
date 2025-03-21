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
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"golang.org/x/sync/errgroup"
)

// Mode determines which directories Walk visits and which directories
// should be updated.
type Mode int

const (
	// In VisitAllUpdateSubdirsMode, Walk visits every directory in the
	// repository. The directories given to Walk and their subdirectories are
	// updated.
	VisitAllUpdateSubdirsMode Mode = iota

	// In VisitAllUpdateDirsMode, Walk visits every directory in the repository.
	// Only the directories given to Walk are updated (not their subdirectories).
	VisitAllUpdateDirsMode

	// In UpdateDirsMode, Walk only visits and updates directories given to Walk.
	// Build files in parent directories are read in order to produce a complete
	// configuration, but the callback is not called for parent directories.
	UpdateDirsMode

	// In UpdateSubdirsMode, Walk visits and updates the directories given to Walk
	// and their subdirectories. Build files in parent directories are read in
	// order to produce a complete configuration, but the callback is not called
	// for parent directories.
	UpdateSubdirsMode
)

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
type WalkFunc func(dir, rel string, c *config.Config, update bool, f *rule.File, subdirs, regularFiles, genFiles []string)

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
func Walk(c *config.Config, cexts []config.Configurer, dirs []string, mode Mode, wf WalkFunc) {
	knownDirectives := make(map[string]bool)
	for _, cext := range cexts {
		for _, d := range cext.KnownDirectives() {
			knownDirectives[d] = true
		}
	}

	updateRels := NewUpdateFilter(c.RepoRoot, dirs, mode)
	ignoreFilter := newIgnoreFilter(c.RepoRoot)

	trie, err := buildTrie(c, updateRels, ignoreFilter)
	if err != nil {
		log.Fatalf("error walking the file system: %v\n", err)
	}

	visit(c, cexts, knownDirectives, updateRels, trie, wf, false)
}

// Recursively traverse a trie to:
//  1. configure (top-down)
//  2. invoke the WalkFunc (bottom-up)
//
// Configuration includes building the config.Config for the directory
// which is inherited by the child directories.
//
// Traversal may skip subtrees or files based on the config.Config exclude/ignore/follow options
// as well as the UpdateFilter callbacks.
func visit(c *config.Config, cexts []config.Configurer, knownDirectives map[string]bool, updateRels *UpdateFilter, trie *pathTrie, wf WalkFunc, updateParent bool) {
	haveError := false

	// Absolute path to the directory being visited
	dir := filepath.Join(c.RepoRoot, trie.rel)

	f, err := trie.build, trie.buildFileErr
	if err != nil {
		log.Print(err)
		if c.Strict {
			// TODO(https://github.com/bazelbuild/bazel-gazelle/issues/1029):
			// Refactor to accumulate and propagate errors to main.
			log.Fatal("Exit as strict mode is on")
		}
		haveError = true
	}

	// Update this directory config.Config with information from the trie
	c.ValidBuildFileNames = trie.walkConfig.validBuildFileNames

	// Configure the current directory if not only collecting files
	configure(cexts, knownDirectives, c, trie.rel, f)

	shouldUpdate := updateRels.shouldUpdate(trie.rel, updateParent)

	// Filter, visit and collect subdirectories
	var subdirs []string
	for _, t := range trie.children {
		if updateRels.shouldVisit(t.rel, shouldUpdate) {
			visit(c.Clone(), cexts, knownDirectives, updateRels, t, wf, shouldUpdate)

			var subdir string
			if trie.rel == "" {
				subdir = t.rel
			} else {
				subdir = t.rel[len(trie.rel)+1:]
			}
			subdirs = append(subdirs, subdir)
		}
	}

	update := !haveError && !trie.walkConfig.ignore && shouldUpdate
	if updateRels.shouldCall(trie.rel, updateParent) {
		genFiles := findGenFiles(trie.walkConfig, f)
		wf(dir, trie.rel, c, update, f, subdirs, trie.files, genFiles)
	}
}

// An UpdateFilter tracks which directories need to be updated
//
// INTERNAL: this is a non-public util only for use within bazel-gazelle.
type UpdateFilter struct {
	mode Mode

	// map from slash-separated paths relative to the
	// root directory ("" for the root itself) to a boolean indicating whether
	// the directory should be updated.
	updateRels map[string]bool
}

// NewUpdateFilter builds a table of prefixes, used to determine which
// directories to update and visit.
//
// root and dirs must be absolute, canonical file paths. Each entry in dirs
// must be a subdirectory of root. The caller is responsible for checking this.
//
// INTERNAL: this is a non-public util only for use within bazel-gazelle.
func NewUpdateFilter(root string, dirs []string, mode Mode) *UpdateFilter {
	relMap := make(map[string]bool)
	for _, dir := range dirs {
		rel, _ := filepath.Rel(root, dir)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}

		i := 0
		for {
			next := strings.IndexByte(rel[i:], '/') + i
			if next-i < 0 {
				relMap[rel] = true
				break
			}
			prefix := rel[:next]
			if _, ok := relMap[prefix]; !ok {
				relMap[prefix] = false
			}
			i = next + 1
		}
	}
	return &UpdateFilter{mode, relMap}
}

// shouldCall returns true if Walk should call the callback in the
// directory rel.
func (u *UpdateFilter) shouldCall(rel string, updateParent bool) bool {
	switch u.mode {
	case VisitAllUpdateSubdirsMode, VisitAllUpdateDirsMode:
		return true
	case UpdateSubdirsMode:
		return updateParent || u.updateRels[rel]
	default: // UpdateDirsMode
		return u.updateRels[rel]
	}
}

// shouldUpdate returns true if Walk should pass true to the callback's update
// parameter in the directory rel. This indicates the build file should be
// updated.
func (u *UpdateFilter) shouldUpdate(rel string, updateParent bool) bool {
	if (u.mode == VisitAllUpdateSubdirsMode || u.mode == UpdateSubdirsMode) && updateParent {
		return true
	}
	return u.updateRels[rel]
}

// shouldVisit returns true if Walk should visit the subdirectory rel.
func (u *UpdateFilter) shouldVisit(rel string, updateParent bool) bool {
	switch u.mode {
	case VisitAllUpdateSubdirsMode, VisitAllUpdateDirsMode:
		return true
	case UpdateSubdirsMode:
		_, ok := u.updateRels[rel]
		return ok || updateParent
	default: // UpdateDirsMode
		_, ok := u.updateRels[rel]
		return ok
	}
}

func loadBuildFile(ctx *buildTrieContext, wc *walkConfig, pkg, dir string, ents []fs.DirEntry) (*rule.File, error) {
	var err error
	readDir := dir
	readEnts := ents
	if ctx.readBuildFilesDir != "" {
		readDir = filepath.Join(ctx.readBuildFilesDir, filepath.FromSlash(pkg))
		readEnts, err = os.ReadDir(readDir)
		if err != nil {
			return nil, err
		}
	}
	path := rule.MatchBuildFile(readDir, wc.validBuildFileNames, readEnts)
	if path == "" {
		return nil, nil
	}
	return rule.LoadFile(path, pkg)
}

func configure(cexts []config.Configurer, knownDirectives map[string]bool, c *config.Config, rel string, f *rule.File) {
	if f != nil {
		for _, d := range f.Directives {
			if !knownDirectives[d.Key] {
				log.Printf("%s: unknown directive: gazelle:%s", f.Path, d.Key)
				if c.Strict {
					// TODO(https://github.com/bazelbuild/bazel-gazelle/issues/1029):
					// Refactor to accumulate and propagate errors to main.
					log.Fatal("Exit as strict mode is on")
				}
			}
		}
	}
	for _, cext := range cexts {
		cext.Configure(c, rel, f)
	}
}

func findGenFiles(wc *walkConfig, f *rule.File) []string {
	if f == nil {
		return nil
	}
	var strs []string
	for _, r := range f.Rules {
		for _, key := range []string{"out", "outs"} {
			if s := r.AttrString(key); s != "" {
				strs = append(strs, s)
			} else if ss := r.AttrStrings(key); len(ss) > 0 {
				strs = append(strs, ss...)
			}
		}
	}

	var genFiles []string
	for _, s := range strs {
		if !wc.isExcluded(path.Join(f.Pkg, s)) {
			genFiles = append(genFiles, s)
		}
	}
	return genFiles
}

func shouldFollow(ctx *buildTrieContext, wc *walkConfig, rel string, ent fs.DirEntry) bool {
	if ent.Type()&os.ModeSymlink == 0 {
		// Not a symlink
		return true
	}
	if !wc.shouldFollow(rel) {
		// A symlink, but not one we should follow.
		return false
	}
	if _, err := os.Stat(path.Join(ctx.rootDir, rel, ent.Name())); err != nil {
		// A symlink, but not one we could resolve.
		return false
	}
	return true
}

// Information lasting the lifetime of the fs walk
type buildTrieContext struct {
	rootDir           string
	readBuildFilesDir string

	// The global/root excludes
	rootExcludes []string

	// The global/initial validBuildFileNames
	rootValidBuildFileNames []string

	// An error group to handle error propagation and concurrent
	eg *errgroup.Group

	// A channel to limit concurrency of IO operations
	limitCh chan struct{}
}

type pathTrie struct {
	rel string

	files    []string
	children []*pathTrie

	walkConfig   *walkConfig
	build        *rule.File
	buildFileErr error

	rw *sync.RWMutex
}

func (trie *pathTrie) addChild(c *pathTrie) *pathTrie {
	trie.rw.Lock()
	defer trie.rw.Unlock()

	trie.children = append(trie.children, c)
	return c
}

func (trie *pathTrie) addFiles(p []string) {
	trie.rw.Lock()
	defer trie.rw.Unlock()

	trie.files = append(trie.files, p...)
}

func (trie *pathTrie) freeze() {
	trie.rw.Lock()
	defer trie.rw.Unlock()

	trie.rw = nil

	slices.Sort(trie.files)
	slices.SortFunc(trie.children, func(a, b *pathTrie) int {
		return strings.Compare(a.rel, b.rel)
	})

	for _, c := range trie.children {
		c.freeze()
	}
}

func buildTrie(c *config.Config, updateRels *UpdateFilter, ignoreFilter *ignoreFilter) (*pathTrie, error) {
	// A channel to limit the number of concurrent goroutines
	//
	// This operation is likely to be limited by memory bandwidth and I/O,
	// not CPU. On a MacBook Pro M1, 6 was the lowest value with best performance,
	// but higher values didn't degrade performance. Higher values may benefit
	// machines with more memory bandwidth.
	//
	// Use BenchmarkWalk to test changes here.
	limitCh := make(chan struct{}, runtime.GOMAXPROCS(0))

	ctx := &buildTrieContext{
		rootDir:                 c.RepoRoot,
		readBuildFilesDir:       c.ReadBuildFilesDir,
		rootValidBuildFileNames: c.ValidBuildFileNames,
		rootExcludes:            c.Exts[walkConfigurerName].(*Configurer).cliExcludes,
		limitCh:                 limitCh,
		eg:                      &errgroup.Group{},
	}

	// An error group to handle error propagation
	trie, err := walkDir(ctx, nil, "", updateRels, ignoreFilter)
	if err != nil {
		return nil, err
	}
	if err := ctx.eg.Wait(); err != nil {
		return nil, err
	}

	// Freeze the full pathTrie only once fully built.
	// TODO: freeze while concurrently walking the fs directories
	trie.freeze()

	return trie, nil
}

// walkDir recursively and concurrently descends into the 'rel' directory and builds a trie
func walkDir(ctx *buildTrieContext, trie *pathTrie, rel string, updateRels *UpdateFilter, ignoreFilter *ignoreFilter) (*pathTrie, error) {
	ctx.limitCh <- struct{}{}
	defer (func() { <-ctx.limitCh })()

	// Absolute path to the directory being visited
	dir := filepath.Join(ctx.rootDir, rel)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var wc *walkConfig
	var t *pathTrie

	// The root initializes the root walkConfig
	if trie == nil {
		t = nil
		wc = &walkConfig{
			validBuildFileNames: ctx.rootValidBuildFileNames,
			excludes:            ctx.rootExcludes,
		}
	} else {
		t = trie
		wc = trie.walkConfig
	}

	build, buildFileErr := loadBuildFile(ctx, wc, rel, dir, entries)

	// A new pathTrie is required: at the root, when a BUILD is found, or when generating a BUILD.
	if trie == nil || build != nil || buildFileErr != nil || !wc.updateOnly {
		// A new pathTrie for this directory, possibly with a new BUILD and configuration.
		t = &pathTrie{
			rel:          rel,
			build:        build,
			buildFileErr: buildFileErr,
			walkConfig:   wc.newChild(rel, build),
			rw:           &sync.RWMutex{},
		}

		// Check if a BUILD excluded itself.
		// Only check if the `walkConfig` contains additional excludes not already checked
		// in the parent config before recursing into the directory.
		if trie == nil || len(t.walkConfig.excludes) > len(trie.walkConfig.excludes) {
			if t.walkConfig.isExcluded(rel) {
				return t, nil
			}
		}

		// Add to the parent trie AFTER checking if excluded
		if trie != nil {
			trie.addChild(t)
		}
	}

	// Collect + recurse entries async to release the limitCh
	ctx.eg.Go(func() error {
		return t.loadEntries(ctx, rel, entries, updateRels, ignoreFilter)
	})

	return t, nil
}

func (trie *pathTrie) loadEntries(ctx *buildTrieContext, rel string, entries []os.DirEntry, updateRels *UpdateFilter, ignoreFilter *ignoreFilter) error {
	eg := &errgroup.Group{}

	// Files collected for this directory. Will be added as single locked operation at end.
	files := []string{}

	// If the trie is a parent directory we must prefix subdirectories
	buildRel := ""
	if rel != trie.rel {
		buildRel = rel[len(trie.rel)+1:]
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryPath := path.Join(rel, entryName)

		// Ignore .git and empty names
		if entryName == "" || entryName == ".git" {
			continue
		}

		if entry.IsDir() {
			// Non-visited directories
			if !updateRels.shouldVisit(entryPath, true) {
				continue
			}

			// Ignored directories
			if ignoreFilter.isDirectoryIgnored(entryPath) {
				continue
			}

			// Directories excluded by config.
			// Performed after `isDirectoryIgnored` + `shouldVisit` which may be faster.
			if trie.walkConfig.isExcluded(entryPath) {
				continue
			}

			// TODO: make potential stat calls async?
			if shouldFollow(ctx, trie.walkConfig, rel, entry) {
				// Asynchrounously walk the subdirectory.
				eg.Go(func() error {
					_, err := walkDir(ctx, trie, entryPath, updateRels, ignoreFilter)
					return err
				})
			}
		} else {
			// Ignored files
			if ignoreFilter.isFileIgnored(entryPath) {
				continue
			}

			// Files excluded by config.
			// Performed after `isFileIgnored` which may be faster.
			if trie.walkConfig.isExcluded(entryPath) {
				continue
			}

			// TODO: make potential stat calls async?
			if shouldFollow(ctx, trie.walkConfig, rel, entry) {
				files = append(files, path.Join(buildRel, entryName))
			}
		}
	}

	trie.addFiles(files)

	return eg.Wait()
}
