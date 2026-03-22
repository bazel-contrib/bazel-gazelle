/* Copyright 2016 The Bazel Authors. All rights reserved.

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

// Package label provides utilities for parsing and manipulating
// Bazel labels. See
// https://docs.bazel.build/versions/master/build-ref.html#labels
// for more information.
package label

import v2 "github.com/bazel-contrib/bazel-gazelle/v2/label"

// A Label represents a label of a build target in Bazel. Labels have three
// parts: a repository name, a package name, and a target name, formatted
// as @repo//pkg:target.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/label.Label instead.
type Label = v2.Label

// New constructs a new label from components.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/label.New instead.
func New(repo, pkg, name string) Label {
	return v2.New(repo, pkg, name)
}

// NoLabel is the zero value of Label. It is not a valid label and may be
// returned when an error occurs.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/label.NoLabel instead.
var NoLabel = v2.NoLabel

// Parse reads a label from a string.
// See https://docs.bazel.build/versions/master/build-ref.html#lexi.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/label.Parse instead.
func Parse(s string) (Label, error) {
	return v2.Parse(s)
}

// ImportPathToBazelRepoName converts a Go import path into a bazel repo name
// following the guidelines in http://bazel.io/docs/be/functions.html#workspace
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/label.ImportPathToBazelRepoName instead.
func ImportPathToBazelRepoName(importpath string) string {
	return v2.ImportPathToBazelRepoName(importpath)
}
