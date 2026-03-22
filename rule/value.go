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

package rule

import (
	v2 "github.com/bazel-contrib/bazel-gazelle/v2/rule"

	bzl "github.com/bazelbuild/buildtools/build"
)

// KeyValue represents a key-value pair. This gets converted into a
// rule attribute, i.e., a Skylark keyword argument.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KeyValue instead.
type KeyValue = v2.KeyValue

// GlobValue represents a Bazel glob expression.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.GlobValue instead.
type GlobValue = v2.GlobValue

// ParseGlobExpr detects whether the given expression is a call to the glob
// function. If it is, ParseGlobExpr returns the glob's patterns and excludes
// (if they are literal strings) and true. If not, ParseGlobExpr returns false.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.ParseGlobExpr instead.
func ParseGlobExpr(e bzl.Expr) (GlobValue, bool) {
	return v2.ParseGlobExpr(e)
}

// BzlExprValue is implemented by types that have custom translations
// to Starlark values.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.BzlExprValue instead.
type BzlExprValue = v2.BzlExprValue

// Merger is implemented by types that can merge their data into an
// existing Starlark expression.
//
// When Merge is invoked, it is responsible for returning a Starlark expression that contains the
// result of merging its data into the previously-existing expression provided as other.
// Note that other can be nil, if no previous attr with this name existed.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.Merger instead.
type Merger = v2.Merger

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.SortedStrings instead.
type SortedStrings = v2.SortedStrings

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.UnsortedStrings instead.
type UnsortedStrings = v2.UnsortedStrings

// SelectStringListValue is a value that can be translated to a Bazel
// select expression that picks a string list based on a string condition.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.SelectStringListValue instead.
type SelectStringListValue = v2.SelectStringListValue

// ExprFromValue converts a value into an expression that can be written into
// a Bazel build file. The following types of values can be converted:
//
//   - bools, integers, floats, strings.
//   - labels (converted to strings).
//   - slices, arrays (converted to lists).
//   - maps (converted to select expressions; keys must be rules in
//     @io_bazel_rules_go//go/platform).
//   - GlobValue (converted to glob expressions).
//   - PlatformStrings (converted to a concatenation of a list and selects).
//
// Converting unsupported types will cause a panic.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.ExprFromValue instead.
func ExprFromValue(val interface{}) bzl.Expr {
	return v2.ExprFromValue(val)
}
