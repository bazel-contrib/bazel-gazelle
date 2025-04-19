package walk

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bazelbuild/bazel-gazelle/rule"
)

// dirInfo holds all the information about a directory that Walk2 needs.
type dirInfo struct {
	// entries holds the contents of the directory. Symbolic links are resolved
	// or not depending on configuration. Excluded and ignored files are included.
	entries []fs.DirEntry

	// subdirs and regularFiles hold the names of subdirectories and regular files
	// that are not ignored or excluded.
	subdirs, regularFiles []string

	// file is the directory's build file. May be nil if the build file doesn't
	// exist or contains errors.
	file *rule.File

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
func (w *walker) loadDirInfo(rel string) (dirInfo, error) {
	var info dirInfo
	var errs []error
	var err error
	dir := filepath.Join(w.rootConfig.RepoRoot, rel)
	info.entries, err = os.ReadDir(dir)
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

	info.file, err = loadBuildFile(parentConfig, w.rootConfig.ReadBuildFilesDir, rel, dir, info.entries)
	if err != nil {
		errs = append(errs, err)
	}

	info.config = configureForWalk(parentConfig, rel, info.file)

	for i, e := range info.entries {
		entryRel := path.Join(rel, e.Name())
		e = resolveFileInfo(info.config, dir, entryRel, e)
		info.entries[i] = e
		if e.IsDir() && !info.config.isExcludedDir(entryRel) {
			info.subdirs = append(info.subdirs, e.Name())
		} else if !e.IsDir() && !info.config.isExcludedFile(entryRel) {
			info.regularFiles = append(info.regularFiles, e.Name())
		}
	}

	return info, errors.Join(errs...)
}

// populateCache loads directory information in a parallel tree traversal.
// This has no semantic effect but should speed up I/O.
//
// populateCache should only be called when recursion is enabled. It attempts
// to avoid traversing excluded subdirectories.
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
		wc := info.config

		for _, subdir := range info.subdirs {
			subdirRel := path.Join(rel, subdir)
			if wc.isExcludedDir(subdirRel) {
				continue
			}
			sem <- struct{}{} // acquire semaphore for child
			wg.Add(1)
			go func() {
				defer wg.Done()
				visit(subdirRel)
			}()
		}
	}

	// Call c.get for all directory prefixes. c.get always requires the parent to
	// be visited first.
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
