package walk

import (
	"fmt"
	"path"
	"sync"

	"github.com/bazelbuild/bazel-gazelle/pathtools"
)

// cache is an in-memory cache for file system information. Its purpose is to
// speed up walking over large directory trees (commonly, the entire repo)
// by parallelizing parts of the walk while still allowing random access
// to parts of the directory tree that haven't been loaded yet.
type cache struct {
	entryMap sync.Map
}

type cacheEntry struct {
	doneC chan struct{}
	info  DirInfo
	err   error
}

// get returns the result of calling the given function on the given key.
//
// If get has not yet been called with the key, it calls load and saves the
// result.
//
// If get was called earlier with the key, it returns the saved result.
//
// get may be called by multiple threads concurrently. Later calls block
// until the result from the first call is ready.
func (c *cache) get(key string, load func(rel string) (DirInfo, error)) (DirInfo, error) {
	// Optimistically load the entry. This is technically unnecessary, but it
	// avoids allocating a new entry in the case where one already exists.
	raw, ok := c.entryMap.Load(key)
	if ok {
		entry := raw.(*cacheEntry)
		<-entry.doneC
		return entry.info, entry.err
	}

	// Create a new entry. Another goroutine may do this concurrently, so if
	// another entry is inserted first, wait on that one.
	entry := &cacheEntry{doneC: make(chan struct{})}
	raw, loaded := c.entryMap.LoadOrStore(key, entry)
	if loaded {
		entry = raw.(*cacheEntry)
		<-entry.doneC
		return entry.info, entry.err
	}

	// Read the directory contents.
	defer close(entry.doneC)
	entry.info, entry.err = load(key)
	return entry.info, entry.err
}

// getLoaded returns the result of a previous call to get with the same key.
// getLoaded panics if get was not called or has not returned yet.
func (c *cache) getLoaded(rel string) (DirInfo, error) {
	e, ok := c.entryMap.Load(rel)
	if ok {
		select {
		case <-e.(*cacheEntry).doneC:
		default:
			ok = false
		}
	}
	if !ok {
		panic(fmt.Sprintf("getLoaded called for %q before it was loaded", rel))
	}
	ce := e.(*cacheEntry)
	return ce.info, ce.err
}

var globalWalker *walker

func setGlobalWalker(w *walker) func() {
	if globalWalker != nil {
		panic("globalWalker already set")
	}
	globalWalker = w
	return func() { globalWalker = nil }
}

// GetDirInfo returns the list of files and subdirectories contained in a
// directory named by rel. It also returns the parsed build file or nil if
// none was present. rel is a slash-separated path, relative to the repository
// root directory or "" for the root directory itself. The returned values
// must not be modified.
//
// GetDirInfo may only be called concurrently with Walk or Walk2. It provides
// access to an internal cache used by those functions. GetDirInfo may
// trigger additional I/O if a directory hasn't been visited yet, but
// its results are cached and shared with Walk or Walk2.
//
// In general, language extensions should prefer to use the RegularFiles,
// Subdirs, and File fields of language.GenerateArgs. This function returns
// the same information and may be used by methods like Resolver.Imports
// that get called earlier without the same information.
func GetDirInfo(rel string) (DirInfo, error) {
	if globalWalker == nil {
		panic("globalWalker is not set")
	}
	rel = path.Clean(rel)

	// Ensure all ancestors are loaded before loading rel itself, since their
	// configuration may exclude rel.
	var prevCfg *walkConfig = nil
	var di DirInfo
	var err error
	pathtools.Prefixes(rel)(func(prefix string) bool {
		if prevCfg != nil && prevCfg.isExcludedDir(prefix) {
			di = DirInfo{}
			err = fmt.Errorf("directory %q is excluded", prefix)
			return false
		}
		di, err = globalWalker.cache.get(prefix, globalWalker.loadDirInfo)
		prevCfg = di.config
		return err == nil
	})
	return di, err
}
