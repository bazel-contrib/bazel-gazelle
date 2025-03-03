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
func visit(c *config.Config, cexts []config.Configurer, knownDirectives map[string]bool, updateRels *UpdateFilter, trie *pathTrie, wf WalkFunc, updateParent bool) ([]string, bool) {
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

	c.ValidBuildFileNames = trie.walkConfig.validBuildFileNames

	// Configure the current directory if not only collecting files
	configure(cexts, knownDirectives, c, trie.rel, f)

	wc := trie.walkConfig

	// Filter and collect files
	regularFiles := make([]string, 0, len(trie.files))
	for entSub := range trie.files {
		regularFiles = append(regularFiles, entSub)
	}

	shouldUpdate := updateRels.shouldUpdate(trie.rel, updateParent)

	// Ensure a deterministic order for regular files and subdirectories.
	// This includes both the params to the callback as well as the `visit` order.
	slices.Sort(regularFiles)

	// TODO: don't sort content of trie, pre-sort it or sort only a local copy
	slices.SortFunc(trie.children, func(a, b *pathTrie) int {
		if a.rel != b.rel {
			return strings.Compare(a.rel, b.rel)
		}
		return strings.Compare(a.entry.Name(), b.entry.Name())
	})

	// Filter and collect subdirectories
	var subdirs []string
	for _, t := range trie.children {
		base := t.entry.Name()
		entRel := path.Join(trie.rel, base)
		if updateRels.shouldVisit(entRel, shouldUpdate) {
			visit(c.Clone(), cexts, knownDirectives, updateRels, t, wf, shouldUpdate)
			subdirs = append(subdirs, base)
		}
	}

	update := !haveError && !wc.ignore && shouldUpdate
	if updateRels.shouldCall(trie.rel, updateParent) {
		genFiles := findGenFiles(wc, f)
		wf(dir, trie.rel, c, update, f, subdirs, regularFiles, genFiles)
	}
	return nil, false
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

func loadBuildFile(readBuildFilesDir string, validBuildFileNames []string, pkg, dir string, ents []fs.DirEntry) (*rule.File, error) {
	var err error
	readDir := dir
	readEnts := ents
	if readBuildFilesDir != "" {
		readDir = filepath.Join(readBuildFilesDir, filepath.FromSlash(pkg))
		readEnts, err = os.ReadDir(readDir)
		if err != nil {
			return nil, err
		}
	}
	path := rule.MatchBuildFile(readDir, validBuildFileNames, readEnts)
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

func resolveFileInfo(wc *walkConfig, dir, rel string, ent fs.DirEntry) fs.DirEntry {
	if ent.Type()&os.ModeSymlink == 0 {
		// Not a symlink, use the original FileInfo.
		return ent
	}
	if !wc.shouldFollow(rel) {
		// A symlink, but not one we should follow.
		return nil
	}
	fi, err := os.Stat(path.Join(dir, ent.Name()))
	if err != nil {
		// A symlink, but not one we could resolve.
		return nil
	}
	return fs.FileInfoToDirEntry(fi)
}

type pathTrie struct {
	rel string

	entry    fs.DirEntry
	files    map[string]fs.DirEntry
	children []*pathTrie

	walkConfig   *walkConfig
	build        *rule.File
	buildFileErr error
}

// Basic factory method to ensure the entry is properly copied
func (trie *pathTrie) newChild(rel string, entry fs.DirEntry) *pathTrie {
	return &pathTrie{
		rel:        path.Join(trie.rel, rel, entry.Name()),
		files:      map[string]fs.DirEntry{},
		entry:      entry,
		walkConfig: trie.walkConfig.newChild(),
	}
}

func buildTrie(c *config.Config, updateRels *UpdateFilter, ignoreFilter *ignoreFilter) (*pathTrie, error) {
	trie := &pathTrie{
		files: map[string]fs.DirEntry{},
		// Initial walk config based on root config.Config and walk.Configurer
		walkConfig: &walkConfig{
			validBuildFileNames: c.ValidBuildFileNames,
			excludes:            c.Exts[walkConfigurerName].(*Configurer).cliExcludes,
			follow:              c.Exts[walkConfigurerName].(*Configurer).cliFollow,
		},
	}

	// A channel to limit the number of concurrent goroutines
	//
	// This operation is likely to be limited by memory bandwidth and I/O,
	// not CPU. On a MacBook Pro M1, 6 was the lowest value with best performance,
	// but higher values didn't degrade performance. Higher values may benefit
	// machines with more memory bandwidth.
	//
	// Use BenchmarkWalk to test changes here.
	limitCh := make(chan struct{}, runtime.GOMAXPROCS(0))

	// An error group to handle error propagation
	eg := errgroup.Group{}
	eg.Go(func() error {
		return trie.walkDir(c.RepoRoot, c.ReadBuildFilesDir, "", "", nil, &eg, limitCh, updateRels, ignoreFilter)
	})

	return trie, eg.Wait()
}

// walkDir recursively and concurrently descends into the 'rel' directory and builds a trie
func (trie *pathTrie) walkDir(root, readBuildFilesDir, rel, buildRel string, entry os.DirEntry, eg *errgroup.Group, limitCh chan struct{}, updateRels *UpdateFilter, ignoreFilter *ignoreFilter) error {
	limitCh <- struct{}{}
	defer (func() { <-limitCh })()

	// Absolute path to the directory being visited
	dir := filepath.Join(root, rel)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	build, buildFileErr := loadBuildFile(readBuildFilesDir, trie.walkConfig.validBuildFileNames, rel, dir, entries)

	if entry != nil && (build != nil || buildFileErr != nil || !trie.walkConfig.updateOnly) {
		child := trie.newChild(buildRel, entry)
		child.build = build
		child.buildFileErr = buildFileErr
		child.walkConfig.readConfig(rel, build)

		// Check if a BUILD excluded itself.
		// Only check if the `readConfig()` added new `excludes` in addition to the parent to avoid
		// simply duplicating the `isExcluded()` already invoked before recursing into `dir`.
		// TODO: could also skip `excludes` from the parent BUILD that were already checked before recursing
		if len(child.walkConfig.excludes) > len(trie.walkConfig.excludes) && child.walkConfig.isExcluded(rel) {
			return nil
		}

		trie.children = append(trie.children, child)
		trie = child
		buildRel = ""
	} else {
		trie.build = build
		trie.buildFileErr = buildFileErr
		trie.walkConfig.readConfig(rel, build)

		if entry != nil {
			buildRel = path.Join(buildRel, entry.Name())
		}
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryPath := path.Join(rel, entryName)

		// Ignore .git and empty names
		if entryName == "" || entryName == ".git" {
			continue
		}

		if entry.IsDir() {
			if ignoreFilter.isDirectoryIgnored(entryPath) {
				continue
			}

			// Ignore directories not even being visited
			if !updateRels.shouldVisit(entryPath, true) {
				continue
			}

			if trie.walkConfig.isExcluded(entryPath) {
				continue
			}

			eg.Go(func() error {
				if ent := resolveFileInfo(trie.walkConfig, dir, entryPath, entry); ent != nil {
					return trie.walkDir(root, readBuildFilesDir, entryPath, buildRel, ent, eg, limitCh, updateRels, ignoreFilter)
				}
				return nil
			})
		} else {
			if ignoreFilter.isFileIgnored(entryPath) {
				continue
			}

			if trie.walkConfig.isExcluded(entryPath) {
				continue
			}

			if ent := resolveFileInfo(trie.walkConfig, dir, entryPath, entry); ent != nil {
				trie.files[path.Join(buildRel, entryName)] = ent
			}
		}
	}
	return nil
}
