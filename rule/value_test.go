/* Copyright 2023 The Bazel Authors. All rights reserved.

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

	"github.com/bazelbuild/bazel-gazelle/label"
	bzl "github.com/bazelbuild/buildtools/build"
	"github.com/google/go-cmp/cmp"
)

func TestExprFromValue(t *testing.T) {
	for name, tt := range map[string]struct {
		val  interface{}
		want bzl.Expr
	}{
		"glob value": {
			val: GlobValue{
				Patterns: []string{"a", "b"},
			},
			want: &bzl.CallExpr{
				X: &bzl.Ident{Name: "glob"},
				List: []bzl.Expr{
					&bzl.ListExpr{
						List: []bzl.Expr{
							&bzl.StringExpr{Value: "a"},
							&bzl.StringExpr{Value: "b"},
						},
					},
				},
			},
		},
		"glob value with excludes": {
			val: GlobValue{
				Patterns: []string{"a", "b"},
				Excludes: []string{"c", "d"},
			},
			want: &bzl.CallExpr{
				X: &bzl.Ident{Name: "glob"},
				List: []bzl.Expr{
					&bzl.ListExpr{
						List: []bzl.Expr{
							&bzl.StringExpr{Value: "a"},
							&bzl.StringExpr{Value: "b"},
						},
					},
					&bzl.AssignExpr{
						LHS: &bzl.Ident{Name: "exclude"},
						Op:  "=",
						RHS: &bzl.ListExpr{
							List: []bzl.Expr{
								&bzl.StringExpr{Value: "c"},
								&bzl.StringExpr{Value: "d"},
							},
						},
					},
				},
			},
		},
		"glob value with allow empty": {
			val: GlobValue{
				Patterns:   []string{"a"},
				AllowEmpty: true,
			},
			want: &bzl.CallExpr{
				X: &bzl.Ident{Name: "glob"},
				List: []bzl.Expr{
					&bzl.ListExpr{
						List: []bzl.Expr{
							&bzl.StringExpr{Value: "a"},
						},
					},
					&bzl.AssignExpr{
						LHS: &bzl.Ident{Name: "allow_empty"},
						Op:  "=",
						RHS: &bzl.Ident{Name: "True"},
					},
				},
			},
		},
		"sorted strings": {
			val: SortedStrings{"@b", ":a", "//:target"},
			want: &bzl.ListExpr{
				List: []bzl.Expr{
					&bzl.StringExpr{Value: ":a"},
					&bzl.StringExpr{Value: "//:target"},
					&bzl.StringExpr{Value: "@b"},
				},
			},
		},
		"unsorted strings": {
			val: UnsortedStrings{"@d", ":a", "//:b"},
			want: &bzl.ListExpr{
				List: []bzl.Expr{
					&bzl.StringExpr{Value: "@d"},
					&bzl.StringExpr{Value: ":a"},
					&bzl.StringExpr{Value: "//:b"},
				},
			},
		},
		"labels": {
			val:  label.New("repo", "pkg", "name"),
			want: &bzl.StringExpr{Value: "@repo//pkg:name"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := ExprFromValue(tt.val)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ExprFromValue() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseGlobExpr(t *testing.T) {
	for _, test := range []struct {
		name, text string
		want       GlobValue
	}{
		{
			name: "empty",
			text: "glob()",
			want: GlobValue{},
		},
		{
			name: "patterns_only",
			text: `glob(["a", "b"])`,
			want: GlobValue{Patterns: []string{"a", "b"}},
		},
		{
			name: "excludes_only",
			text: `glob(exclude = ["x", "y"])`,
			want: GlobValue{Excludes: []string{"x", "y"}},
		},
		{
			name: "patterns_before_excludes",
			text: `glob(["a", "b"], exclude = ["x", "y"])`,
			want: GlobValue{Patterns: []string{"a", "b"}, Excludes: []string{"x", "y"}},
		},
		{
			name: "excludes_before_patterns",
			text: `glob(exclude = ["x", "y"], ["a", "b"])`,
			want: GlobValue{Excludes: []string{"x", "y"}},
		},
		{
			name: "patterns_nonliteral",
			text: `glob(["a", B])`,
			want: GlobValue{Patterns: []string{"a"}},
		},
		{
			name: "excludes_nonliteral",
			text: `glob(exclude = ["x", Y])`,
			want: GlobValue{Excludes: []string{"x"}},
		},
		{
			name: "other_args",
			text: `glob(["a"], allow_empty = True, exclude_directories = 1)`,
			want: GlobValue{Patterns: []string{"a"}, AllowEmpty: true},
		},
		{
			name: "allow_empty_only",
			text: `glob([], allow_empty = True)`,
			want: GlobValue{Patterns: []string{}, AllowEmpty: true},
		},
		{
			name: "allow_empty_false",
			text: `glob(["a"], allow_empty = False)`,
			want: GlobValue{Patterns: []string{"a"}},
		},
		{
			name: "invalid_args",
			text: `glob(["a"], ["b"], exclude = ["x"], unknown = 1, *args, **kwargs)`,
			want: GlobValue{Patterns: []string{"a"}, Excludes: []string{"b"}},
		},
		{
			name: "positional_patterns_and_excludes",
			text: `glob(["a"], ["b"])`,
			want: GlobValue{Patterns: []string{"a"}, Excludes: []string{"b"}},
		},
		{
			name: "reordered_named_patterns_and_excludes",
			text: `glob(exclude = ["a"], include = ["x"])`,
			want: GlobValue{Patterns: []string{"x"}, Excludes: []string{"a"}},
		},
		{
			name: "positional_ident_patterns",
			text: `glob(include, ["b"])`,
			want: GlobValue{Excludes: []string{"b"}},
		},
		{
			name: "positional_ident_excludes",
			text: `glob(["a"], exclude, ["foo"])`,
			want: GlobValue{Patterns: []string{"a"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, err := bzl.ParseDefault(test.name, []byte(test.text))
			if err != nil {
				t.Fatal(err)
			}
			e := f.Stmt[0]
			glob, ok := ParseGlobExpr(e)
			if !ok {
				t.Fatal("not a glob expression")
			}
			if diff := cmp.Diff(glob, test.want); diff != "" {
				t.Errorf("glob (-got, +want):\n%s", diff)
			}
		})
	}
}
