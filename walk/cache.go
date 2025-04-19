package walk

import (
	"fmt"
	"sync"
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
	info  dirInfo
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
func (c *cache) get(key string, load func(rel string) (dirInfo, error)) (dirInfo, error) {
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
func (c *cache) getLoaded(rel string) (dirInfo, error) {
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
