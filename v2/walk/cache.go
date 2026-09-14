/* Copyright 2026 The Bazel Authors. All rights reserved.

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
	"fmt"
	"path"
	"sync"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/pathtools"
)

// Cache is an in-memory cache for file system information. Its purpose is to
// speed up walking over large directory trees (commonly, the entire repo)
// by parallelizing parts of the walk while still allowing random access
// to parts of the directory tree that haven't been loaded yet.
type Cache struct {
	rootConfig *config.Config
	entryMap   sync.Map
}

func NewCache(rootConfig *config.Config) *Cache {
	return &Cache{rootConfig: rootConfig}
}

// GetDirInfo returns the list of files and subdirectories contained in a
// directory named by rel. It also returns the parsed build file or nil if
// none was present. rel is a slash-separated path, relative to the repository
// root directory or "" for the root directory itself. The returned values
// must not be modified.
//
// When writing extension methods like Generate that need metadata about files
// outside the directory where they're called, prefer calling GetDirInfo over
// raw file I/O. Since Cache saves file metadata in memory, calling GetDirInfo
// is often much faster. GetDirInfo also respects `gazelle:exclude` and
// similar directives, so extensions have the same view of the file system
// as the rest of Gazelle.
func (c *Cache) GetDirInfo(rel string) (DirInfo, error) {
	rel = path.Clean(rel)
	if rel == "." {
		rel = ""
	}

	// Ensure all ancestors are loaded before loading rel itself, since their
	// configuration may exclude rel.
	var prevCfg *walkConfig = nil
	for prefix := range pathtools.Prefixes(rel) {
		if prevCfg != nil && prevCfg.isExcludedDir(prefix) {
			return DirInfo{}, fmt.Errorf("directory %q is excluded", prefix)
		}
		di, err := c.get(prefix)
		if err != nil {
			return DirInfo{}, err
		}
		prevCfg = di.config
	}
	return c.get(rel)
}

type cacheEntry struct {
	doneC chan struct{}
	info  DirInfo
	err   error
}

// get returns the result of calling c.load on the given key.
//
// If get was not called yet with the key, it calls load and saves the result.
//
// If get was called earlier with the key, it returns the saved result.
//
// get may be called by multiple threads concurrently. Later calls block
// until the result from the first call is ready.
func (c *Cache) get(key string) (DirInfo, error) {
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
	entry.info, entry.err = c.load(key)
	return entry.info, entry.err
}

// getLoaded returns the result of a previous call to get with the same key.
// getLoaded panics if get was not called or has not returned yet.
func (c *Cache) getLoaded(key string) (DirInfo, error) {
	e, ok := c.entryMap.Load(key)
	if ok {
		select {
		case <-e.(*cacheEntry).doneC:
		default:
			ok = false
		}
	}
	if !ok {
		panic(fmt.Sprintf("getLoaded called for %q before it was loaded", key))
	}
	ce := e.(*cacheEntry)
	return ce.info, ce.err
}
