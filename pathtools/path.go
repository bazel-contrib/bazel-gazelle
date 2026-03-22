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

// Package pathtools provides utilities for manipulating paths.  Most paths
// within Gazelle are slash-separated paths, relative to the repository root
// directory. The repository root directory is represented by the empty
// string. Paths in this format may be used directly as package names in labels.
package pathtools

import v2 "github.com/bazel-contrib/bazel-gazelle/v2/pathtools"

// HasPrefix returns whether the slash-separated path p has the given
// prefix. Unlike strings.HasPrefix, this function respects component
// boundaries, so "/home/foo" is not a prefix is "/home/foobar/baz". If the
// prefix is empty, this function always returns true.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/pathtools.HasPrefix instead.
//go:fix inline
func HasPrefix(p, prefix string) bool {
	return v2.HasPrefix(p, prefix)
}

// TrimPrefix returns p without the provided prefix. If p doesn't start
// with prefix, it returns p unchanged. Unlike strings.HasPrefix, this function
// respects component boundaries (assuming slash-separated paths), so
// TrimPrefix("foo/bar", "foo") returns "baz".
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/pathtools.TrimPrefix instead.
//go:fix inline
func TrimPrefix(p, prefix string) string {
	return v2.TrimPrefix(p, prefix)
}

// RelBaseName returns the base name for rel, a slash-separated path relative
// to the repository root. If rel is empty, RelBaseName returns the base name
// of prefix. If prefix is empty, RelBaseName returns the base name of root,
// the absolute file path of the repository root directory. If that's empty
// to, then RelBaseName returns "root".
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/pathtools.RelBaseName instead.
//go:fix inline
func RelBaseName(rel, prefix, root string) string {
	return v2.RelBaseName(rel, prefix, root)
}

// Index returns the starting index of the first ocurrence of the string sub
// within the slash-separated path p. sub must start and end at component
// boundaries within p.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/pathtools.Index instead.
//go:fix inline
func Index(p, sub string) int {
	return v2.Index(p, sub)
}

// LastIndex returns the starting index of the last occurrence of the string sub
// within the slash-separated path p. sub must start and end at component
// boundaries within p.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/pathtools.LastIndex instead.
//go:fix inline
func LastIndex(p, sub string) int {
	return v2.LastIndex(p, sub)
}

// Prefixes returns an iterator (iter.Seq) over all the prefixes of p.
// For example, if p is "a/b/c", the iterator yields "", "a", "a/b", "a/b/c".
//
// p must be a slash-separated path. It may be relative or absolute. p
// does not need to be a clean path, but if it is not clean, Prefixes ignores
// redundant slashes while keeping redundant path elements. For example,
// if p is "a/../b//c/", the iterator yields "a", "..", "b", "c".
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/pathtools.Prefixes instead.
//go:fix inline
func Prefixes(p string) func(yield func(string) bool) {
	return v2.Prefixes(p)
}
