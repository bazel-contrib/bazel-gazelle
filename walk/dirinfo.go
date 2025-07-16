package walk

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	if info.config.isExcludedDir(rel) {
		// Build file excludes the current directory. Ignore contents.
		entries = nil
	}

	for _, e := range entries {
		entryRel := path.Join(rel, e.Name())
		e = maybeResolveSymlink(info.config, dir, entryRel, e)
		if e.IsDir() && !info.config.isExcludedDir(entryRel) {
			info.Subdirs = append(info.Subdirs, e.Name())
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

	return info, errors.Join(errs...)
}

// populateCache loads directory information in a parallel tree traversal.
// This has no semantic effect but should speed up I/O.
//
// populateCache should only be called when recursion is enabled. It avoids
// traversing excluded subdirectories.
func (w *walker) populateCache(rels []string) {
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

		for _, subdir := range info.Subdirs {
			subdirRel := path.Join(rel, subdir)
			sem <- struct{}{} // acquire semaphore for child
			wg.Add(1)
			go func() {
				defer wg.Done()
				visit(subdirRel)
			}()
		}
	}

	// Load each directory prefix. walker.loadDirInfo requires the parent
	// directory to be visited first so its configuration is known.
	w.cache.get("", w.loadDirInfo)
	for _, dir := range rels {
		slash := 0
		for {
			i := strings.Index(dir[slash:], "/")
			if i < 0 {
				break
			}
			prefix := dir[:slash+i]
			slash = slash + i + 1
			w.cache.get(prefix, w.loadDirInfo)
		}
	}

	// Visit the directories recursively in parallel.
	for _, dir := range rels {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			visit(dir)
		}()
	}

	wg.Wait()
}
