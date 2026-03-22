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

// Package rule provides tools for editing Bazel build files. It is intended to
// be a more powerful replacement for
// github.com/bazelbuild/buildtools/build.Rule, adapted for Gazelle's usage. It
// is language agnostic, but it may be used for language-specific rules by
// providing configuration.
//
// File is the primary interface to this package. A File represents an
// individual build file. It comprises a list of Rules and a list of Loads.
// Rules and Loads may be inserted, modified, or deleted. When all changes
// are done, File.Save() may be called to write changes back to a file.
package rule

import (
	"io/fs"
	"os"

	v2 "github.com/bazel-contrib/bazel-gazelle/v2/rule"

	bzl "github.com/bazelbuild/buildtools/build"
)

// File provides editing functionality for a build file. You can create a
// new file with EmptyFile or load an existing file with LoadFile. After
// changes have been made, call Save to write changes back to a file.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.File instead.
//go:fix inline
type File = v2.File

// EmptyFile creates a File wrapped around an empty syntax tree.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.EmptyFile instead.
//go:fix inline
func EmptyFile(path, pkg string) *File {
	return v2.EmptyFile(path, pkg)
}

// LoadFile loads a build file from disk, parses it, and scans for rules and
// load statements. The syntax tree within the returned File will be modified
// by editing methods.
//
// This function returns I/O and parse errors without modification. It's safe
// to use os.IsNotExist and similar predicates.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.LoadFile instead.
//go:fix inline
func LoadFile(path, pkg string) (*File, error) {
	return v2.LoadFile(path, pkg)
}

// LoadWorkspaceFile is similar to LoadFile but parses the file as a WORKSPACE
// file.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.LoadWorkspaceFile instead.
//go:fix inline
func LoadWorkspaceFile(path, pkg string) (*File, error) {
	return v2.LoadWorkspaceFile(path, pkg)
}

// LoadMacroFile loads a bzl file from disk, parses it, then scans for the load
// statements and the rules called from the given Starlark function. If there is
// no matching function name, then a new function with that name will be created.
// The function's syntax tree will be returned within File and can be modified by
// Sync and Save calls.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.LoadMacroFile instead.
//go:fix inline
func LoadMacroFile(path, pkg, defName string) (*File, error) {
	return v2.LoadMacroFile(path, pkg, defName)
}

// EmptyMacroFile creates a bzl file at the given path and within the file creates
// a Starlark function with the provided name. The function can then be modified
// by Sync and Save calls.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.EmptyMacroFile instead.
//go:fix inline
func EmptyMacroFile(path, pkg, defName string) (*File, error) {
	return v2.EmptyMacroFile(path, pkg, defName)
}

// LoadData parses a build file from a byte slice and scans it for rules and
// load statements. The syntax tree within the returned File will be modified
// by editing methods.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.LoadData instead.
//go:fix inline
func LoadData(path, pkg string, data []byte) (*File, error) {
	return v2.LoadData(path, pkg, data)
}

// LoadWorkspaceData is similar to LoadData but parses the data as a
// WORKSPACE file.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.LoadWorkspaceData instead.
//go:fix inline
func LoadWorkspaceData(path, pkg string, data []byte) (*File, error) {
	return v2.LoadWorkspaceData(path, pkg, data)
}

// LoadMacroData parses a bzl file from a byte slice and scans for the load
// statements and the rules called from the given Starlark function. If there is
// no matching function name, then a new function will be created, and added to the
// File the next time Sync is called. The function's syntax tree will be returned
// within File and can be modified by Sync and Save calls.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.LoadMacroData instead.
//go:fix inline
func LoadMacroData(path, pkg, defName string, data []byte) (*File, error) {
	return v2.LoadMacroData(path, pkg, defName, data)
}

// ScanAST creates a File wrapped around the given syntax tree. This tree
// will be modified by editing methods.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.ScanAST instead.
//go:fix inline
func ScanAST(pkg string, bzlFile *bzl.File) *File {
	return v2.ScanAST(pkg, bzlFile)
}

// ScanASTBody creates a File wrapped around the given syntax tree. It will also
// scan the AST for a function matching the given defName, and if the function
// does not exist it will create a new one and mark it to be added to the File
// the next time Sync is called.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.ScanASTBody instead.
//go:fix inline
func ScanASTBody(pkg, defName string, bzlFile *bzl.File) *File {
	return v2.ScanASTBody(pkg, defName, bzlFile)
}

// MatchBuildFile looks for a file in files that has a name from names.
// If there is at least one matching file, a path will be returned by joining
// dir and the first matching name. If there are no matching files, the
// empty string is returned.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.MatchBuildFile instead.
//go:fix inline
func MatchBuildFile(dir string, names []string, ents []fs.DirEntry) string {
	return v2.MatchBuildFile(dir, names, ents)
}

// Deprecated: Prefer MatchBuildFile, it's more efficient to fetch a []fs.DirEntry
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.MatchBuildFileName instead.
//go:fix inline
func MatchBuildFileName(dir string, names []string, files []os.FileInfo) string {
	return v2.MatchBuildFileName(dir, names, files)
}

// Load represents a load statement within a build file.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.Load instead.
//go:fix inline
type Load = v2.Load

// NewLoad creates a new, empty load statement for the given file name.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.NewLoad instead.
//go:fix inline
func NewLoad(name string) *Load {
	return v2.NewLoad(name)
}

// Rule represents a rule statement within a build file.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.Rule instead.
//go:fix inline
type Rule = v2.Rule

// NewRule creates a new, empty rule with the given kind and name.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.NewRule instead.
//go:fix inline
func NewRule(kind, name string) *Rule {
	return v2.NewRule(kind, name)
}

// ShouldKeep returns whether e is marked with a "# keep" comment. Kept
// expressions should not be removed or modified.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.ShouldKeep instead.
//go:fix inline
func ShouldKeep(e bzl.Expr) bool {
	return v2.ShouldKeep(e)
}

// CheckInternalVisibility overrides the given visibility if the package is
// internal.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.CheckInternalVisibility instead.
//go:fix inline
func CheckInternalVisibility(rel, visibility string) string {
	return v2.CheckInternalVisibility(rel, visibility)
}
