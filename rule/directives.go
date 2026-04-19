/* Copyright 2017 The Bazel Authors. All rights reserved.

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

package rule

import (
	v2 "github.com/bazel-contrib/bazel-gazelle/v2/rule"

	bzl "github.com/bazelbuild/buildtools/build"
)

// Directive is a key-value pair extracted from a top-level comment in
// a build file. Directives have the following format:
//
//	# gazelle:key value
//
// Keys may not contain spaces. Values may be empty and may contain spaces,
// but surrounding space is trimmed.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.Directive instead.
//
//go:fix inline
type Directive = v2.Directive

// ParseDirectives scans f for Gazelle directives. The full list of directives
// is returned. Errors are reported for unrecognized directives and directives
// out of place (after the first statement).
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.ParseDirectives instead.
//
//go:fix inline
func ParseDirectives(f *bzl.File) []Directive {
	return v2.ParseDirectives(f)
}

// ParseDirectivesFromMacro scans a macro body for Gazelle directives. The
// full list of directives is returned. Errors are reported for unrecognized
// directives and directives out of place (after the first statement).
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.ParseDirectivesFromMacro instead.
//
//go:fix inline
func ParseDirectivesFromMacro(f *bzl.DefStmt) []Directive {
	return v2.ParseDirectivesFromMacro(f)
}

// ParseDirectivesFromFile reads a file and extracts Gazelle directives from it.
// Each line is matched against the same pattern used for BUILD file comments
// (# gazelle:key value). Blank lines and comment lines that don't match
// the directive pattern are ignored.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.ParseDirectivesFromFile instead.
//
//go:fix inline
func ParseDirectivesFromFile(filePath string) ([]Directive, error) {
	return v2.ParseDirectivesFromFile(filePath)
}
