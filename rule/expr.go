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

package rule

import (
	v2 "github.com/bazel-contrib/bazel-gazelle/v2/rule"

	bzl "github.com/bazelbuild/buildtools/build"
)

// MapExprStrings applies a function to string sub-expressions within e.
// An expression containing the results with the same structure as e is
// returned.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.MapExprStrings instead.
func MapExprStrings(e bzl.Expr, f func(string) string) bzl.Expr {
	return v2.MapExprStrings(e, f)
}

// FlattenExpr takes an expression that may have been generated from
// PlatformStrings and returns its values in a flat, sorted, de-duplicated
// list. Comments are accumulated and de-duplicated across duplicate
// expressions. If the expression could not have been generted by
// PlatformStrings, the expression will be returned unmodified.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.FlattenExpr instead.
func FlattenExpr(e bzl.Expr) bzl.Expr {
	return v2.FlattenExpr(e)
}
