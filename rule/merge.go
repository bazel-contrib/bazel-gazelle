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

// MergeRules copies information from src into dst, usually discarding
// information in dst when they have the same attributes.
//
// If dst is marked with a "# keep" comment, either above the rule or as
// a suffix, nothing will be changed.
//
// If src has an attribute that is not in dst, it will be copied into dst.
//
// If src and dst have the same attribute and the attribute is mergeable and the
// attribute in dst is not marked with a "# keep" comment, values in the dst
// attribute not marked with a "# keep" comment will be dropped, and values from
// src will be copied in.
//
// If dst has an attribute not in src, and the attribute is mergeable and not
// marked with a "# keep" comment, values in the attribute not marked with
// a "# keep" comment will be dropped. If the attribute is empty afterward,
// it will be deleted.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.MergeRules instead.
func MergeRules(src, dst *Rule, mergeable map[string]bool, filename string) {
	v2.MergeRules(src, dst, mergeable, filename)
}

// RemoveNoopKeepComments controls whether comments with "# keep" are removed when they are not needed.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.RemoveNoopKeepComments instead.
var RemoveNoopKeepComments = v2.RemoveNoopKeepComments

// MergeList merges two bzl.ListExpr of strings. The lists are merged in the
// following way:
//
//   - If a string appears in both lists, it appears in the result.
//   - If a string appears in only src list, it appears in the result.
//   - If a string appears in only dst list, it is dropped from the result.
//   - If a string appears in neither list, it is dropped from the result.
//
// The result is nil if both lists are nil or empty.
//
// If the result is non-nil, it will have ForceMultiLine set if either of the
// input lists has ForceMultiLine set or if any of the strings in the result
// have a "# keep" comment.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.MergeList instead.
func MergeList(srcExpr, dstExpr bzl.Expr) *bzl.ListExpr {
	return v2.MergeList(srcExpr, dstExpr)
}

// MergeDict merges two bzl.DictExpr, src and dst, where the keys are strings
// and the values are lists of strings.
//
// If both src and dst are non-nil, the keys in src are merged into dst. If both
// src and dst have the same key, the values are merged using MergeList.
// If the same key is present in both src and dst, and the values are not compatible,
// an error is returned.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.MergeDict instead.
func MergeDict(srcExpr, dstExpr bzl.Expr) (*bzl.DictExpr, error) {
	return v2.MergeDict(srcExpr, dstExpr)
}

// SquashRules copies information from src into dst without discarding
// information in dst. SquashRules detects duplicate elements in lists and
// dictionaries, but it doesn't sort elements after squashing. If squashing
// fails because the expression is not understood, an error is returned,
// and neither rule is modified.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.SquashRules instead.
func SquashRules(src, dst *Rule, filename string) error {
	return v2.SquashRules(src, dst, filename)
}
