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
	"errors"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
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
//
// DEPRECATED: Use Walk2Func with Walk2 instead.
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
//
// DEPRECATED: Use Walk2 instead.
func Walk(c *config.Config, cexts []config.Configurer, dirs []string, mode Mode, wf WalkFunc) {
	w2f := func(args Walk2FuncArgs) Walk2FuncResult {
		wf(args.Dir, args.Rel, args.Config, args.Update, args.File, args.Subdirs, args.RegularFiles, args.GenFiles)
		return Walk2FuncResult{}
	}
	err := Walk2(c, cexts, dirs, mode, w2f)
	if err != nil {
		log.Print(err)
		if c.Strict {
			log.Fatal("Exit as strict mode is on")
		}
	}
}

type Walk2Func func(args Walk2FuncArgs) Walk2FuncResult

type Walk2FuncArgs struct {
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

type Walk2FuncResult struct {
	// Err is an error encountered by the callback function. It's logged to the
	// console. When Config.Strict is set, setting Err causes Walk2 to return
	// early.
	Err error

	// RelsToVisit is a list of additional directories to visit. Each directory is
	// a slash-separated path, relative to the repository root or "" for the root
	// directory itself.
	//
	// These directories will be visited after the directories the walk was
	// already going to visit. They will not be visited more than once in total.
	// When one of these directories is visited, the Walk2Args.Update flag will
	// be false unless the directory was already going to be visited with the
	// Update flag true as part of the walk.
	//
	// This list may contain non-existent directories.
	RelsToVisit []string
}

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
func Walk2(c *config.Config, cexts []config.Configurer, dirs []string, mode Mode, wf Walk2Func) error {
	w, err := newWalker(c, cexts, dirs, mode, wf)
	if err != nil {
		return err
	}

	// Do the main tree walk, visiting directories the user requested.
	w.visit(c, "", false)
	if c.Strict && len(w.errs) > 0 {
		return errors.Join(w.errs...)
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
	cache *cache

	// cexts is a list of configuration extensions, provided by the caller.
	cexts []config.Configurer

	// knownDirectives is a list of directives supported by those extensions.
	knownDirectives map[string]bool

	// mode determines how directories are visited, provided by the caller.
	mode Mode

	// shouldUpdateRel indicates whether we should update a set of directories
	// named by slash-separated repo-root-relative paths. The set is generated
	// from the list of directories passed in to Walk2. This map contains true
	// for explicitly listed directories, and false for ancestor directories
	// that are not explicitly listed.
	shouldUpdateRel map[string]bool

	// wf is the callback provided by the caller. It's called in each directory
	// that needs to be updated or indexed, determined by mode.
	wf Walk2Func

	// visits holds a record of each time visit was called, keyed by
	// slash-separated repo-root-relative path. It prevents visiting
	// the same directory more than once and tracks information that's needed
	// by parents.
	visits map[string]visitInfo

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

func newWalker(c *config.Config, cexts []config.Configurer, dirs []string, mode Mode, wf Walk2Func) (*walker, error) {
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
		cache:           new(cache),
		cexts:           cexts,
		knownDirectives: knownDirectives,
		mode:            mode,
		wf:              wf,
		shouldUpdateRel: shouldUpdateRel,
		visits:          make(map[string]visitInfo),
	}
	if mode == VisitAllUpdateSubdirsMode || mode == UpdateSubdirsMode {
		w.populateCache(rels)
	}

	return w, nil
}

// shouldVisit returns whether the visit method should be called on rel.
// We always need to visit directories requested by the caller and their
// parents. We may also need to visit subdirectories.
func (w *walker) shouldVisit(rel string, parentConfig *walkConfig, updateParent bool) bool {
	if _, ok := w.visits[rel]; ok {
		// Already visited.
		return false
	}
	if parentConfig.isExcludedDir(rel) {
		// Excluded directory.
		return false
	}

	switch w.mode {
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

// shouldCall returns whether the caller's Walk2Func callback should be called
// on rel. We always need to call it on directories requested by the caller.
// We may need to call it on their subdirectories, depending on mode. We also
// need to call it on any additional directories requested by the callback.
func (w *walker) shouldCall(rel string, updateParent bool) bool {
	if w.visits[rel].containedByParent {
		return false
	}
	switch w.mode {
	case VisitAllUpdateSubdirsMode, VisitAllUpdateDirsMode:
		return true
	case UpdateSubdirsMode:
		return updateParent || w.shouldUpdateRel[rel]
	default: // UpdateDirsMode
		return w.shouldUpdateRel[rel]
	}
}

// shouldUpdate returns true if Walk should pass true to the callback's update
// parameter in the directory rel. This indicates the build file should be
// updated.
func (w *walker) shouldUpdate(rel string, updateParent bool) bool {
	if (w.mode == VisitAllUpdateSubdirsMode || w.mode == UpdateSubdirsMode) && updateParent {
		return true
	}
	return w.shouldUpdateRel[rel]
}

// visit is the main recursive function of walker. It visits one directory,
// possibly recurses into subdirectories, and possible calls the callback.
//
// updateParent should indicate whether the the current mode tells Gazelle
// to call the callback in the parent directory with update = true (see
// shouldUpdate). The callback may not actually be called if the build file
// contains syntax errors or a gazelle:ignore directive.
func (w *walker) visit(c *config.Config, rel string, updateParent bool) {
	w.visits[rel] = visitInfo{c: c} // to be updated when we're further along.

	// Absolute path to the directory being visited
	dir := filepath.Join(c.RepoRoot, rel)

	// Load the build file.
	info, err := w.cache.get(rel, w.loadDirInfo)
	if err != nil {
		w.errs = append(w.errs, err)
	}
	hasBuildFileError := err != nil

	wc := info.config
	containedByParent := info.file == nil && wc.updateOnly
	if !containedByParent {
		configure(w.cexts, w.knownDirectives, c, rel, info.file, info.config)
	}
	regularFiles := info.regularFiles
	subdirs := info.subdirs

	w.visits[rel] = visitInfo{
		c:                 c,
		containedByParent: containedByParent,
		regularFiles:      regularFiles,
		subdirs:           subdirs,
	}

	if wc.isExcludedDir(rel) {
		return
	}

	// Visit subdirectories, as needed.
	shouldUpdate := w.shouldUpdate(rel, updateParent)
	for _, subdir := range subdirs {
		subdirRel := path.Join(rel, subdir)
		if w.shouldVisit(subdirRel, info.config, shouldUpdate) {
			w.visit(c.Clone(), subdirRel, shouldUpdate)
			if c.Strict && len(w.errs) > 0 {
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
	}

	// Call the callback to update this directory.
	update := !wc.ignore && shouldUpdate && !hasBuildFileError
	if w.shouldCall(rel, updateParent) {
		genFiles := findGenFiles(wc, info.file)
		result := w.wf(Walk2FuncArgs{
			Dir:          dir,
			Rel:          rel,
			Config:       c,
			Update:       update,
			File:         info.file,
			Subdirs:      subdirs,
			RegularFiles: regularFiles,
			GenFiles:     genFiles,
		})
		if result.Err != nil {
			w.errs = append(w.errs, result.Err)
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

func configure(cexts []config.Configurer, knownDirectives map[string]bool, c *config.Config, rel string, f *rule.File, wc *walkConfig) {
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
