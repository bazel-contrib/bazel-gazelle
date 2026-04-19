package walk

import v2 "github.com/bazel-contrib/bazel-gazelle/v2/walk"

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
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/walk.GetDirInfo instead.
func GetDirInfo(rel string) (DirInfo, error) {
	return v2.GetDirInfo(rel)
}
