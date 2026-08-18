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
	"slices"
	"strings"

	bzl "github.com/bazelbuild/buildtools/build"
)

// sortExprLabels sorts lists of strings using the same order as buildifier.
// Buildifier also sorts string lists, but not those involved with "select"
// expressions. This function is intended to be used with bzl.Walk.
func sortExprLabels(e bzl.Expr, _ []bzl.Expr) {
	list, ok := e.(*bzl.ListExpr)
	if !ok || len(list.List) < 2 {
		return
	}

	// Check that all elements are strings
	for _, elem := range list.List {
		if _, ok := elem.(*bzl.StringExpr); !ok {
			return // don't sort lists unless all elements are strings
		}
	}

	// A comment block above the first element is pinned to the top of the
	// list, matching buildifier.
	before := list.List[0].Comment().Before
	list.List[0].Comment().Before = nil

	slices.SortStableFunc(list.List, compareStringExpr)

	list.List[0].Comment().Before = append(before, list.List[0].Comment().Before...)
}

// Code below this point matches the sort order of
// github.com/bazelbuild/buildtools/build/rewrite.go

// compareStringExpr compares two string literals to be sorted. The strings
// are first grouped into four phases: most strings, strings beginning with
// ":", strings beginning with "//", and strings beginning with "@". The next
// significant part of the comparison is the list of elements in the value,
// where elements are split at `.' and `:'. Finally we compare by value,
// leaving equal values in their original order.
func compareStringExpr(a, b bzl.Expr) int {
	sa := a.(*bzl.StringExpr).Value
	sb := b.(*bzl.StringExpr).Value

	if phaseA, phaseB := labelPhase(sa), labelPhase(sb); phaseA != phaseB {
		return phaseA - phaseB
	}

	return compareStringExpValue(sa, sb)
}

func labelPhase(s string) int {
	switch {
	case strings.HasPrefix(s, ":"):
		return 1
	case strings.HasPrefix(s, "//"):
		return 2
	case strings.HasPrefix(s, "@"):
		return 3
	}
	return 0
}

// compareStringExpValue compares the `.'/`:' separated segments of two
// values without splitting them: a separator ends a segment, so it sorts
// before any other character, and `.' and `:' compare as equal. Values with
// equal segments are ordered by raw value.
func compareStringExpValue(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			sepA := a[i] == '.' || a[i] == ':'
			sepB := b[i] == '.' || b[i] == ':'
			if sepA != sepB {
				if sepA {
					return -1
				}
				return 1
			}
			if !sepA {
				if a[i] < b[i] {
					return -1
				}
				return 1
			}
			// Both are separators, which compare as equal.
		}
	}

	if len(a) != len(b) {
		return len(a) - len(b)
	}

	// The values differ only by separators.
	return strings.Compare(a, b)
}
