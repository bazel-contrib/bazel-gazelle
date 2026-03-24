package walk

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/bazelbuild/bazel-gazelle/rule"
)

// DirInfo holds all the information about a directory that Walk2 needs.
type DirInfo struct {
	// Subdirs and RegularFiles hold the names of subdirectories and regular files
	// that are not ignored or excluded.
	// GenFiles is a list of generated files, named in "out" or "outs" attributes
	// of targets in the directory's build file.
	// The content of these slices must not be modified.
	Subdirs, RegularFiles, GenFiles []string

	// traversalSubdirs holds subdirectories that should be traversed, including
	// excluded directories that may contain explicitly included files.
	traversalSubdirs []string

	// traversable reports whether this directory should be visited at all.
	traversable bool

	// File is the directory's build File. May be nil if the build File doesn't
	// exist or contains errors.
	File *rule.File

	// config is the configuration used by Configurer. We may precompute this
	// before Configure is called to parallelize directory traversal without
	// visiting excluded subdirectories.
	config *walkConfig
}

// loadDirInfo reads directory info for the directory named by the given
// slash-separated path relative to the repo root.
//
// Do not call this method directly. This should be used with w.cache.get to
// avoid redundant I/O.
//
// loadDirInfo must be called on the parent directory first and the result
// must be stored in the cache unless rel is "" (repo root).
//
// This method may return partial results with an error. For example, if the
// directory's build file contains a syntax error, the contents of the
// directory are still returned.
func (w *walker) loadDirInfo(rel string) (DirInfo, error) {
	var info DirInfo
	var errs []error
	var err error
	dir := filepath.Join(w.rootConfig.RepoRoot, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		errs = append(errs, err)
	}

	var parentConfig *walkConfig
	if rel == "" {
		parentConfig = getWalkConfig(w.rootConfig)
	} else {
		parentRel := path.Dir(rel)
		if parentRel == "." {
			parentRel = ""
		}
		parentInfo, _ := w.cache.getLoaded(parentRel)
		parentConfig = parentInfo.config
	}

	info.File, err = loadBuildFile(parentConfig, w.rootConfig.ReadBuildFilesDir, rel, dir, entries)
	if err != nil {
		errs = append(errs, err)
	}

	info.config = configureForWalk(parentConfig, rel, info.File)
	// A directory may be excluded from its parent's visible Subdirs but still
	// need traversal if a later include re-adds something below it.
	info.traversable = w.shouldTraverseDir(info.config, dir, rel, entries, info.File, make(map[string]struct{}))
	if !info.traversable {
		// Build file excludes the current directory. Ignore contents.
		entries = nil
	}

	for _, e := range entries {
		entryRel := path.Join(rel, e.Name())
		e = maybeResolveSymlink(info.config, dir, entryRel, e)
		if e.IsDir() && w.shouldTraverseDir(info.config, filepath.Join(dir, e.Name()), entryRel, nil, nil, make(map[string]struct{})) {
			info.traversalSubdirs = append(info.traversalSubdirs, e.Name())
			if !info.config.isExcludedDir(entryRel) {
				info.Subdirs = append(info.Subdirs, e.Name())
			}
		} else if !e.IsDir() && !info.config.isExcludedFile(entryRel) {
			info.RegularFiles = append(info.RegularFiles, e.Name())
		}
	}

	info.GenFiles = findGenFiles(info.config, info.File)

	// Reduce cap of each slice to len, so that if the caller appends, they'll
	// need to copy to a new backing array. This is defensive: it prevents
	// multiple callers from overwriting the same backing array.
	info.RegularFiles = info.RegularFiles[:len(info.RegularFiles):len(info.RegularFiles)]
	info.Subdirs = info.Subdirs[:len(info.Subdirs):len(info.Subdirs)]
	info.GenFiles = info.GenFiles[:len(info.GenFiles):len(info.GenFiles)]
	info.traversalSubdirs = info.traversalSubdirs[:len(info.traversalSubdirs):len(info.traversalSubdirs)]

	return info, errors.Join(errs...)
}

// shouldTraverseDir reports whether rel must be visited at all.
//
// This is intentionally different from "is rel itself visible?".
// An excluded directory still needs traversal when ordered path directives
// re-include a descendant beneath it. In that case, rel stays out of the
// public Subdirs list, but it remains in traversalSubdirs so the walker can
// reach the included descendant.
func (w *walker) shouldTraverseDir(wc *walkConfig, dir, rel string, entries []fs.DirEntry, file *rule.File, seen map[string]struct{}) bool {
	if path.Base(rel) == ".git" || wc.ignoreFilter.isDirectoryIgnored(rel) {
		return false
	}
	// Visible directories are always traversed.
	if !wc.isExcludedByPathDirectives(rel) {
		return true
	}
	if _, ok := seen[rel]; ok {
		return false
	}
	seen[rel] = struct{}{}
	defer delete(seen, rel)

	if entries == nil {
		var err error
		entries, err = os.ReadDir(dir)
		if err != nil {
			return false
		}
	}
	if file == nil {
		file, _ = loadBuildFile(wc, w.rootConfig.ReadBuildFilesDir, rel, dir, entries)
	}
	// Generated outputs make the directory observable even when the directory
	// path itself is excluded.
	if len(findGenFiles(wc, file)) > 0 {
		return true
	}

	// Search excluded subtrees for any descendant that survives the ordered
	// include/exclude directives.
	for _, e := range entries {
		entryRel := path.Join(rel, e.Name())
		e = maybeResolveSymlink(wc, dir, entryRel, e)
		if e.IsDir() {
			if w.shouldTraverseDir(wc, filepath.Join(dir, e.Name()), entryRel, nil, nil, seen) {
				return true
			}
			continue
		}
		if !wc.isExcludedFile(entryRel) {
			return true
		}
	}
	return false
}

// populateCache loads directory information in a parallel tree traversal.
// This has no semantic effect but should speed up I/O.
//
// populateCache should only be called when recursion is enabled. It avoids
// traversing excluded subdirectories.
func (w *walker) populateCache(mode Mode) {
	// sem is a semaphore.
	//
	// Acquiring the semaphore by sending struct{}{} grants permission to spawn
	// goroutine to visit a subdirectory.
	//
	// Each goroutine releases the semaphore for itself before acquiring it again
	// for each child. This prevents a deadlock that could occur for a deeply
	// nested series of directories.
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	var visit func(string)
	visit = func(rel string) {
		info, err := w.cache.get(rel, w.loadDirInfo)
		<-sem // release semaphore for self
		if err != nil {
			return
		}

		for _, subdir := range info.traversalSubdirs {
			subdirRel := path.Join(rel, subdir)

			// Navigate to the subdirectory if it should be visited.
			if w.shouldVisit(mode, subdirRel, true) {
				sem <- struct{}{} // acquire semaphore for child
				wg.Add(1)
				go func() {
					defer wg.Done()
					visit(subdirRel)
				}()
			}
		}
	}

	// Start the traversal at the root directory.
	sem <- struct{}{}
	visit("")

	wg.Wait()
}
