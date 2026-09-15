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
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/pathtools"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
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

type WalkFunc func(args WalkFuncArgs) (WalkFuncResult, error)

type WalkFuncArgs struct {
	// Dir is the absolute file system path to the directory being visited.
	Dir string

	// rel is the relative slash-separated path to the directory from the
	// repository root. Will be "" for the repository root directory itself.
	Rel string

	// Config is the configuration for the current directory. This may have been
	// modified by directives in the directory's build file.
	Config *config.Config

	// Update is true when the build file may be updated.
	Update bool

	// File is the existing build file in the directory. Will be nil if there
	// was no file.
	File *rule.File

	// Subdirs is a list of names of subdirectories within dir, not
	// including excluded files. A directory is listed here regardless of
	// whether the subdirectory contains (or will contain) a build file.
	// If the update_only generation mode is enabled, this list also contains
	// recursive subdirectories, up to and including those at the edge of the
	// same Bazel package.
	Subdirs []string

	// RegularFiles is a list of names of regular files within dir, not
	// including excluded files. Symbolic links to files and non-followed
	// directories are included in this list. If the update_only generation mode
	// is enabled, this list also contains files from recursive subdirectories
	// within the same Bazel package (those that can be matched by glob).
	RegularFiles []string

	// GenFiles is a list of names of generated files, found by reading
	// "out" and "outs" attributes of rules in f.
	GenFiles []string
}

type WalkFuncResult struct {
	// RelsToVisit is a list of additional directories to visit. Each directory is
	// a slash-separated path, relative to the repository root or "" for the root
	// directory itself.
	//
	// These directories will be visited after the directories the walk was
	// already going to visit. They will not be visited more than once in total.
	// When one of these directories is visited, the WalkFuncArgs.Update flag will
	// be false unless the directory was already going to be visited with the
	// Update flag true as part of the walk.
	//
	// This list may contain non-existent directories.
	RelsToVisit []string
}

// Walk traverses a limited part of the directory tree rooted at c.RepoRoot
// and calls the function wf in each visited directory.
//
// The dirs and mode parameters determine which directories Walk visits.
// Walk calls wf in each directory in dirs with the WalkFuncArgs.Update
// flag set to true. This indicates Gazelle should update build files in that
// directory. Depending on the mode flag, Walk may additionally visit
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
// wf may return an error with its result. In strict mode (c.Strict), this
// causes Walk to return early.
func Walk(
	ctx context.Context,
	c *config.Config,
	cexts []config.Configurer,
	cache *Cache,
	dirs []string,
	mode Mode,
	wf WalkFunc) error {

	if err := ctx.Err(); err != nil {
		return err
	}
	w, err := newWalker(c, cexts, cache, dirs, mode, wf)
	if err != nil {
		return err
	}

	// Do the main tree walk, visiting directories the user requested.
	w.visit(ctx, mode, c, "", false)
	if err := w.shouldStop(ctx); err != nil {
		return err
	}

	// Visit additional directories that extensions requested for indexing.
	// Don't visit subdirectories recursively, even when recursion is enabled.
	for len(w.relsToVisit) > 0 {
		// Don't simply range over relsToVisit. We may append more.
		relToVisit := w.relsToVisit[0]
		w.relsToVisit = w.relsToVisit[1:]

		// Make sure to visit prefixes of relToVisit as well so we apply
		// configuration directives.
		if path.IsAbs(relToVisit) {
			panic(fmt.Sprintf("relToVisit must not be absolute: %q", relToVisit))
		}
		if relToVisit != "" && relToVisit != path.Clean(relToVisit) {
			panic(fmt.Sprintf("relToVisit is not clean: %q", relToVisit))
		}
		for rel := range pathtools.Prefixes(relToVisit) {
			if _, ok := w.visits[rel]; !ok {
				// Never visited this directory.
				parentRel := path.Dir(rel)
				if parentRel == "." {
					parentRel = ""
				}
				parentCfg := w.visits[parentRel].c
				if getWalkConfig(parentCfg).isExcludedDir(rel) {
					break
				}
				if _, err := w.cache.get(rel); err != nil {
					// Error loading directory. Most commonly, this is because the
					// directory doesn't exist, but it could actually be a file
					// or we don't have permission, or some other I/O error.
					// Skip it.
					break
				}
				c := parentCfg.Clone()
				w.visit(ctx, UpdateDirsMode, c, rel, false)
				if err := w.shouldStop(ctx); err != nil {
					return err
				}
			}
		}
	}
	return errors.Join(w.errs...)
}

// walker holds state needed for a walk of the source tree.
type walker struct {
	// repoRoot is the absolute file path to the repo's root directory.
	repoRoot string

	// rootConfig is the configuration for the repo root directory.
	rootConfig *config.Config

	// cache provides access to directory information.
	cache *Cache

	// cexts is a list of configuration extensions, provided by the caller.
	cexts []config.Configurer

	// knownDirectives is a list of directives supported by those extensions.
	knownDirectives map[string]bool

	// shouldUpdateRel indicates whether we should update a set of directories
	// named by slash-separated repo-root-relative paths. The set is generated
	// from the list of directories passed in to Walk. This map contains true
	// for explicitly listed directories, and false for ancestor directories
	// that are not explicitly listed.
	shouldUpdateRel map[string]bool

	// wf is the callback provided by the caller. It's called in each directory
	// that needs to be updated or indexed, determined by mode.
	wf WalkFunc

	// visits holds a record of each time visit was called, keyed by
	// slash-separated repo-root-relative path. It prevents visiting
	// the same directory more than once and tracks information that's needed
	// by parents.
	visits map[string]visitInfo

	// relsToVisit is a list of slash-separated repo-root-relative paths to
	// additional directories to visit. These directories are not visited
	// recursively. wf is called with WalkFuncArgs.Update false.
	relsToVisit []string

	// relsToVisitSeen indicates whether a string was added to relsToVisit.
	// It's used to avoid appending a path more than once.
	relsToVisitSeen map[string]struct{}

	// errs is a list of errors encountered while walking the directory tree.
	// If the Config.Strict flag is set in the root configuration, we return
	// quickly after the first error.
	errs []error
}

type visitInfo struct {
	// containedByParent is true if the directory does not (and should not)
	// contain a build file. The parent directory may use regularFiles
	// and subdirs.
	containedByParent bool

	c                     *config.Config
	regularFiles, subdirs []string
}

func newWalker(c *config.Config, cexts []config.Configurer, cache *Cache, dirs []string, mode Mode, wf WalkFunc) (*walker, error) {
	knownDirectives := make(map[string]bool)
	for _, cext := range cexts {
		for _, d := range cext.KnownDirectives() {
			knownDirectives[d] = true
		}
	}

	rels := make([]string, len(dirs))
	for i, dir := range dirs {
		rel, err := filepath.Rel(c.RepoRoot, dir)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		rels[i] = rel
	}

	shouldUpdateRel := make(map[string]bool)
	for _, rel := range rels {
		i := 0
		for {
			next := strings.IndexByte(rel[i:], '/') + i
			if next-i < 0 {
				shouldUpdateRel[rel] = true
				break
			}
			prefix := rel[:next]
			if _, ok := shouldUpdateRel[prefix]; !ok {
				shouldUpdateRel[prefix] = false
			}
			i = next + 1
		}
	}

	w := &walker{
		repoRoot:        c.RepoRoot,
		rootConfig:      c,
		cache:           cache,
		cexts:           cexts,
		knownDirectives: knownDirectives,
		wf:              wf,
		shouldUpdateRel: shouldUpdateRel,
		visits:          make(map[string]visitInfo),
		relsToVisitSeen: make(map[string]struct{}),
	}

	// Asynchronously populate the walker cache in the background.
	go w.populateCache(mode)

	return w, nil
}

// shouldVisit returns whether the visit method should be called on rel.
// We always need to visit directories requested by the caller and their
// parents. We may also need to visit subdirectories.
func (w *walker) shouldVisit(mode Mode, rel string, updateParent bool) bool {
	switch mode {
	case VisitAllUpdateSubdirsMode, VisitAllUpdateDirsMode:
		return true
	case UpdateSubdirsMode:
		_, ok := w.shouldUpdateRel[rel]
		return ok || updateParent
	default: // UpdateDirsMode
		_, ok := w.shouldUpdateRel[rel]
		return ok
	}
}

// shouldUpdate returns true if Walk should pass true to the callback's update
// parameter in the directory rel. This indicates the build file should be
// updated.
func (w *walker) shouldUpdate(mode Mode, rel string, updateParent bool) bool {
	if (mode == VisitAllUpdateSubdirsMode || mode == UpdateSubdirsMode) && updateParent {
		return true
	}
	return w.shouldUpdateRel[rel]
}

// shouldStop must be called after each call to w.visit. It returns an error
// if we should stop the walk without calling w.visit again. This error may
// be returned to Walk's caller.
func (w *walker) shouldStop(ctx context.Context) error {
	if ctx.Err() != nil && (len(w.errs) == 0 || !errors.Is(w.errs[len(w.errs)-1], ctx.Err())) {
		// Ensure the context cancellation is the last error so it's clear why the
		// walk stopped.
		w.errs = append(w.errs, ctx.Err())
	}
	if ctx.Err() != nil || (len(w.errs) > 0 && w.rootConfig.Strict) {
		return errors.Join(w.errs...)
	}
	return nil
}

// visit is the main recursive function of walker. It visits one directory,
// possibly recurses into subdirectories, and possible calls the callback.
//
// updateParent should indicate whether the the current mode tells Gazelle
// to call the callback in the parent directory with update = true (see
// shouldUpdate). The callback may not actually be called if the build file
// contains syntax errors or a gazelle:ignore directive.
func (w *walker) visit(ctx context.Context, mode Mode, c *config.Config, rel string, updateParent bool) {
	// Absolute path to the directory being visited
	dir := filepath.Join(c.RepoRoot, rel)

	// Load the build file and directory metadata.
	info, err := w.cache.get(rel)
	if err != nil {
		w.errs = append(w.errs, err)
	}
	hasBuildFileError := err != nil
	wc := info.config

	if wc.isExcludedDir(rel) {
		return
	}

	containedByParent := info.File == nil && wc.updateOnly

	// Configure the directory, if we haven't done so already.
	_, alreadyConfigured := w.visits[rel]
	if !containedByParent && !alreadyConfigured {
		if err := configure(ctx, w.cexts, w.knownDirectives, c, rel, info.File, info.config); err != nil {
			w.errs = append(w.errs, err)
		}
	}

	regularFiles := info.RegularFiles
	subdirs := info.Subdirs
	shouldUpdate := w.shouldUpdate(mode, rel, updateParent)
	w.visits[rel] = visitInfo{
		c:                 c,
		containedByParent: containedByParent,
		regularFiles:      regularFiles,
		subdirs:           subdirs,
	}

	// Visit subdirectories, as needed.
	for _, subdir := range subdirs {
		subdirRel := path.Join(rel, subdir)
		if w.shouldVisit(mode, subdirRel, shouldUpdate) {
			w.visit(ctx, mode, c.Clone(), subdirRel, shouldUpdate)
			if err := w.shouldStop(ctx); err != nil {
				return
			}
		}
	}

	// Recursively collect regular files from subdirectories that won't contain
	// build files. Files are added in depth-first pre-order.
	if !containedByParent {
		var collect func(string, string)
		collect = func(rel, prefix string) {
			vi := w.visits[rel]
			if !vi.containedByParent {
				return
			}
			for _, f := range vi.regularFiles {
				regularFiles = append(regularFiles, path.Join(prefix, f))
			}
			for _, f := range vi.subdirs {
				subdirs = append(subdirs, path.Join(prefix, f))
			}
			for _, subdir := range vi.subdirs {
				collect(path.Join(rel, subdir), path.Join(prefix, subdir))
			}
		}
		for _, subdir := range subdirs {
			collect(path.Join(rel, subdir), subdir)
		}

		// Call the callback to update this directory.
		update := !wc.ignore && shouldUpdate && !hasBuildFileError
		result, err := w.wf(WalkFuncArgs{
			Dir:          dir,
			Rel:          rel,
			Config:       c,
			Update:       update,
			File:         info.File,
			Subdirs:      subdirs,
			RegularFiles: regularFiles,
			GenFiles:     info.GenFiles,
		})
		if err != nil {
			w.errs = append(w.errs, err)
		}
		for _, relToVisit := range result.RelsToVisit {
			// Normalize RelsToVisit to clean relative paths and convert root "."
			// to an empty string.
			relToVisit = path.Clean(relToVisit)
			if relToVisit == "." {
				relToVisit = ""
			}

			if _, ok := w.relsToVisitSeen[relToVisit]; !ok {
				w.relsToVisit = append(w.relsToVisit, relToVisit)
				w.relsToVisitSeen[relToVisit] = struct{}{}
			}
		}
	}
}

func loadBuildFile(wc *walkConfig, readBuildFilesDir string, pkg, dir string, ents []fs.DirEntry) (*rule.File, error) {
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
	path := rule.MatchBuildFile(readDir, wc.validBuildFileNames, readEnts)
	if path == "" {
		return nil, nil
	}
	return rule.LoadFile(path, pkg)
}

func configure(
	ctx context.Context,
	cexts []config.Configurer,
	knownDirectives map[string]bool,
	c *config.Config,
	rel string,
	f *rule.File,
	wc *walkConfig) error {
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
	c.Exts[walkNameCached] = wc
	var errs []error
	for _, cext := range cexts {
		if err := cext.Configure(ctx, config.ConfigureArgs{
			Config: c,
			Rel:    rel,
			File:   f,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
		if !wc.isExcludedFile(path.Join(f.Pkg, s)) {
			genFiles = append(genFiles, s)
		}
	}
	return genFiles
}

// maybeResolveSymlink conditionally resolves a symbolic link.
//
// If ent is a symbolic link and Gazelle is configured to follow it (with
// # gazelle:follow), then maybeResolveSymlink resolves the link and returns it.
// The returned entry has the original name, but other metadata describes
// the target file or directory.
//
// Otherwise, maybeResolveSymlink returns ent as-is.
func maybeResolveSymlink(wc *walkConfig, dir, rel string, ent fs.DirEntry) fs.DirEntry {
	if ent.Type()&os.ModeSymlink == 0 {
		// Not a symlink, use the original FileInfo.
		return ent
	}
	if !wc.shouldFollow(rel) {
		// A symlink, but not one we should follow.
		return ent
	}
	fi, err := os.Stat(path.Join(dir, ent.Name()))
	if err != nil {
		// A symlink, but not one we could resolve.
		return ent
	}
	return fs.FileInfoToDirEntry(fi)
}
