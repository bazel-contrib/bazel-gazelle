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
	"testing"

	bzl "github.com/bazelbuild/buildtools/build"
)

func sortAndFormat(t *testing.T, src string) string {
	t.Helper()
	f, err := bzl.ParseBuild("BUILD.bazel", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range f.Stmt {
		bzl.Walk(stmt, sortExprLabels)
	}
	return string(bzl.Format(f))
}

// Comments attached to list elements move with their element when the list is
// sorted, including a comment block above the first element.
func TestSortExprLabelsComments(t *testing.T) {
	for name, tc := range map[string]struct {
		src, want string
	}{
		"before comment on first element moves with it": {
			src: `deps = [
    # comment on b
    ":b",
    ":a",
]
`,
			want: `deps = [
    # comment on b
    ":a",
    ":b",
]
`,
		},
		"before comment on middle element moves with it": {
			src: `deps = [
    ":c",
    # comment on b
    ":b",
    ":a",
]
`,
			want: `deps = [
    ":a",
    # comment on b
    ":b",
    ":c",
]
`,
		},
		"suffix comments move with elements": {
			src: `deps = [
    ":b",  # comment on b
    ":a",  # comment on a
]
`,
			want: `deps = [
    ":a",  # comment on a
    ":b",  # comment on b
]
`,
		},
		"already sorted list with leading comment unchanged": {
			src: `deps = [
    # comment on a
    ":a",
    ":b",
]
`,
			want: `deps = [
    # comment on a
    ":a",
    ":b",
]
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := sortAndFormat(t, tc.src)
			if got != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

// The buildifier sort order splits values on "." and ":", comparing the
// resulting segments, so a separator sorts before any other character and
// "." and ":" compare equal, with ties broken by raw value then input order.
func TestSortExprLabelsOrdering(t *testing.T) {
	for name, tc := range map[string]struct {
		src, want string
	}{
		"separator sorts before dash": {
			src: `deps = [
    ":foo-bar",
    ":foo.bar",
]
`,
			want: `deps = [
    ":foo.bar",
    ":foo-bar",
]
`,
		},
		"separator sorts before plus": {
			src: `deps = [
    ":a+b",
    ":a.b",
]
`,
			want: `deps = [
    ":a.b",
    ":a+b",
]
`,
		},
		"colon separator sorts before digits": {
			src: `deps = [
    ":a5",
    ":a:2",
]
`,
			want: `deps = [
    ":a:2",
    ":a5",
]
`,
		},
		"dot and colon separators tie broken by raw value": {
			src: `deps = [
    ":a:b",
    ":a.b",
]
`,
			want: `deps = [
    ":a.b",
    ":a:b",
]
`,
		},
		"duplicate values keep input order": {
			src: `deps = [
    ":a",  # first
    ":a",  # second
]
`,
			want: `deps = [
    ":a",  # first
    ":a",  # second
]
`,
		},
		"relative phase sorts before absolute": {
			src: `deps = [
    "//x",
    "/x",
]
`,
			want: `deps = [
    "/x",
    "//x",
]
`,
		},
		"empty string sorts first": {
			src: `deps = [
    "x",
    "",
    ":a",
]
`,
			want: `deps = [
    "",
    "x",
    ":a",
]
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := sortAndFormat(t, tc.src)
			if got != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
