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
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	bzl "github.com/bazelbuild/buildtools/build"
)

// This file contains tests for File, Load, Rule, and related functions.
// Tests only cover some basic functionality and a few non-obvious cases.
// Most test coverage will come from clients of this package.

func TestEditAndSync(t *testing.T) {
	old := []byte(`
load("a.bzl", "x_library")

x_library(name = "foo")

load("b.bzl", y_library = "y")

y_library(name = "bar")
`)
	f, err := LoadData(filepath.Join("old", "BUILD.bazel"), "", old)
	if err != nil {
		t.Fatal(err)
	}

	loadA := f.Loads[0]
	loadA.Delete()
	loadB := f.Loads[1]
	loadB.Add("x_library")
	loadB.Remove("y_library")
	loadC := NewLoad("c.bzl")
	loadC.AddAlias("z_library", "z")
	loadC.Add("y_library")
	loadC.Insert(f, 3)

	foo := f.Rules[0]
	foo.Delete()
	bar := f.Rules[1]
	bar.SetAttr("srcs", []string{"bar.y"})
	loadMaybe := NewLoad("//some:maybe.bzl")
	loadMaybe.Add("maybe")
	loadMaybe.Insert(f, 0)
	baz := NewRule("maybe", "baz")
	baz.AddArg(&bzl.LiteralExpr{Token: "z"})
	baz.AddArg(&bzl.LiteralExpr{Token: "z"})
	if err := baz.UpdateArg(0, &bzl.LiteralExpr{Token: "z0"}); err != nil {
		t.Fatal(err)
	}
	if err := baz.UpdateArg(10, &bzl.LiteralExpr{Token: "blah"}); err == nil {
		t.Fatalf("want error because tried to modify an arg outside of arg bounds, got nil")
	}
	baz.SetAttr("srcs", GlobValue{
		Patterns: []string{"**"},
		Excludes: []string{"*.pem"},
	})
	baz.Insert(f)

	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
load("//some:maybe.bzl", "maybe")
load("b.bzl", "x_library")
load(
    "c.bzl",
    "y_library",
    z = "z_library",
)

y_library(
    name = "bar",
    srcs = ["bar.y"],
)

maybe(
    z0,
    z,
    name = "baz",
    srcs = glob(
        ["**"],
        exclude = ["*.pem"],
    ),
)
`)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPassInserted(t *testing.T) {
	old := []byte(`
load("a.bzl", "baz")

def foo():
    go_repository(name = "bar")
`)
	f, err := LoadMacroData(filepath.Join("old", "repo.bzl"), "", "foo", old)
	if err != nil {
		t.Fatal(err)
	}

	f.Rules[0].Delete()
	f.Sync()
	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
load("a.bzl", "baz")

def foo():
    pass
`)

	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestPassRemoved(t *testing.T) {
	old := []byte(`
load("a.bzl", "baz")

def foo():
    pass
`)
	f, err := LoadMacroData(filepath.Join("old", "repo.bzl"), "", "foo", old)
	if err != nil {
		t.Fatal(err)
	}

	bar := NewRule("go_repository", "bar")
	bar.Insert(f)
	f.Sync()
	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
load("a.bzl", "baz")

def foo():
    go_repository(name = "bar")
`)

	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestFunctionInserted(t *testing.T) {
	f, err := LoadMacroData(filepath.Join("old", "repo.bzl"), "", "foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	bar := NewRule("go_repository", "bar")
	bar.Insert(f)
	f.Sync()
	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
def foo():
    go_repository(name = "bar")
`)

	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestArgsAlwaysEndUpBeforeKwargs(t *testing.T) {
	f, err := LoadData(filepath.Join("old", "BUILD.bazel"), "", nil)
	if err != nil {
		t.Fatal(err)
	}

	bar := NewRule("maybe", "bar")
	bar.SetAttr("url", "https://doesnotexist.com")
	bar.AddArg(&bzl.Ident{Name: "http_archive"})
	bar.Insert(f)
	f.Sync()
	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
maybe(
    http_archive,
    name = "bar",
    url = "https://doesnotexist.com",
)
`)

	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestDeleteSyncDelete(t *testing.T) {
	old := []byte(`
x_library(name = "foo")

# comment

x_library(name = "bar")
`)
	f, err := LoadData(filepath.Join("old", "BUILD.bazel"), "", old)
	if err != nil {
		t.Fatal(err)
	}

	foo := f.Rules[0]
	bar := f.Rules[1]
	foo.Delete()
	f.Sync()
	bar.Delete()
	f.Sync()
	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`# comment`)
	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestInsertDeleteSync(t *testing.T) {
	old := []byte("")
	f, err := LoadData(filepath.Join("old", "BUILD.bazel"), "", old)
	if err != nil {
		t.Fatal(err)
	}

	foo := NewRule("filegroup", "test")
	foo.Insert(f)
	foo.Delete()
	f.Sync()
	got := strings.TrimSpace(string(f.Format()))
	want := ""
	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestSymbolsReturnsKeys(t *testing.T) {
	f, err := LoadData(filepath.Join("load", "BUILD.bazel"), "", []byte(`load("a.bzl", "y", z = "a")`))
	if err != nil {
		t.Fatal(err)
	}
	got := f.Loads[0].Symbols()
	want := []string{"y", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v; want %#v", got, want)
	}
}

func TestLoadCommentsAreRetained(t *testing.T) {
	f, err := LoadData(filepath.Join("load", "BUILD.bazel"), "", []byte(`
load(
    "a.bzl",
    # Comment for a symbol that will be deleted.
    "baz",
    # Some comment without remapping.
    "foo",
    # Some comment with remapping.
    my_bar = "bar",
)
`))
	if err != nil {
		t.Fatal(err)
	}
	l := f.Loads[0]
	l.Remove("baz")
	f.Sync()
	l.Add("new_baz")
	f.Sync()

	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
load(
    "a.bzl",
    # Some comment without remapping.
    "foo",
    "new_baz",
    # Some comment with remapping.
    my_bar = "bar",
)
`)

	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestKeepRule(t *testing.T) {
	for _, tc := range []struct {
		desc, src string
		want      bool
	}{
		{
			desc: "prefix",
			src: `
# keep
x_library(name = "x")
`,
			want: true,
		}, {
			desc: "prefix with description",
			src: `
# keep: hack, see more in ticket #42
x_library(name = "x")
`,
			want: true,
		}, {
			desc: "compact_suffix",
			src: `
x_library(name = "x") # keep
`,
			want: true,
		}, {
			desc: "multiline_internal",
			src: `
x_library( # keep
    name = "x",
)
`,
			want: false,
		}, {
			desc: "multiline_suffix",
			src: `
x_library(
    name = "x",
) # keep
`,
			want: true,
		}, {
			desc: "after",
			src: `
x_library(name = "x")
# keep
`,
			want: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := LoadData(filepath.Join(tc.desc, "BUILD.bazel"), "", []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if got := f.Rules[0].ShouldKeep(); got != tc.want {
				t.Errorf("got %v; want %v", got, tc.want)
			}
		})
	}
}

func TestShouldKeepExpr(t *testing.T) {
	for _, tc := range []struct {
		desc, src string
		path      func(e bzl.Expr) bzl.Expr
		want      bool
	}{
		{
			desc: "before",
			src: `
# keep
"s"
`,
			want: true,
		}, {
			desc: "before with description",
			src: `
# keep: we need it for the ninja feature
"s"
`,
			want: true,
		}, {
			desc: "before but not the correct prefix (keeping)",
			src: `
			# keeping this for now
"s"
`,
			want: false,
		}, {
			desc: "before but not the correct prefix (no colon)",
			src: `
			# keep this around for the time being
"s"
`,
			want: false,
		}, {
			desc: "suffix",
			src: `
"s" # keep
`,
			want: true,
		}, {
			desc: "after",
			src: `
"s"
# keep
`,
			want: false,
		}, {
			desc: "list_elem_prefix",
			src: `
[
    # keep
    "s",
]
`,
			path: func(e bzl.Expr) bzl.Expr { return e.(*bzl.ListExpr).List[0] },
			want: true,
		}, {
			desc: "list_elem_suffix",
			src: `
[
    "s", # keep
]
`,
			path: func(e bzl.Expr) bzl.Expr { return e.(*bzl.ListExpr).List[0] },
			want: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ast, err := bzl.Parse(tc.desc, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			expr := ast.Stmt[0]
			if tc.path != nil {
				expr = tc.path(expr)
			}
			got := ShouldKeep(expr)
			if got != tc.want {
				t.Errorf("got %v; want %v", got, tc.want)
			}
		})
	}
}

func TestInternalVisibility(t *testing.T) {
	tests := []struct {
		rel      string
		expected string
	}{
		{rel: "internal", expected: "//:__subpackages__"},
		{rel: "a/b/internal", expected: "//a/b:__subpackages__"},
		{rel: "a/b/internal/c", expected: "//a/b:__subpackages__"},
		{rel: "a/b/internal/c/d", expected: "//a/b:__subpackages__"},
		{rel: "a/b/internal/c/internal", expected: "//a/b/internal/c:__subpackages__"},
		{rel: "a/b/internal/c/internal/d", expected: "//a/b/internal/c:__subpackages__"},
	}

	for _, tt := range tests {
		if actual := CheckInternalVisibility(tt.rel, "default"); actual != tt.expected {
			t.Errorf("got %v; want %v", actual, tt.expected)
		}
	}
}

func TestSortLoadsByName(t *testing.T) {
	f, err := LoadMacroData(
		filepath.Join("third_party", "repos.bzl"),
		"", "repos",
		[]byte(`load("@bazel_gazelle//:deps.bzl", "go_repository")
load("//some:maybe.bzl", "maybe")
load("//some2:maybe.bzl", "maybe2")
load("//some1:maybe4.bzl", "maybe1")
load("b.bzl", "x_library")
def repos():
    go_repository(
        name = "com_github_bazelbuild_bazel_gazelle",
    )
`))
	if err != nil {
		t.Error(err)
	}
	sort.Stable(loadsByName{
		loads: f.Loads,
		exprs: f.File.Stmt,
	})
	f.Sync()

	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
load("@bazel_gazelle//:deps.bzl", "go_repository")
load("//some:maybe.bzl", "maybe")
load("//some1:maybe4.bzl", "maybe1")
load("//some2:maybe.bzl", "maybe2")
load("b.bzl", "x_library")

def repos():
    go_repository(
        name = "com_github_bazelbuild_bazel_gazelle",
    )
`)

	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestSortRulesByKindAndName(t *testing.T) {
	f, err := LoadMacroData(
		filepath.Join("third_party", "repos.bzl"),
		"", "repos",
		[]byte(`load("@bazel_gazelle//:deps.bzl", "go_repository")
def repos():
    some_other_call_rule()
    go_repository(
        name = "com_github_andybalholm_cascadia",
    )
    go_repository(
        name = "com_github_bazelbuild_buildtools",
    )
    go_repository(
        name = "com_github_bazelbuild_rules_go",
    )
    go_repository(
        name = "com_github_bazelbuild_bazel_gazelle",
    )
    a_rule(
        name = "name1",
    )
`))
	if err != nil {
		t.Error(err)
	}
	sort.Stable(rulesByKindAndName{
		rules: f.Rules,
		exprs: f.function.stmt.Body,
	})
	repos := []string{
		"name1",
		"com_github_andybalholm_cascadia",
		"com_github_bazelbuild_bazel_gazelle",
		"com_github_bazelbuild_buildtools",
		"com_github_bazelbuild_rules_go",
	}
	for i, r := range repos {
		rule := f.Rules[i]
		if rule.Name() != r {
			t.Errorf("expect rule %s at %d, got %s", r, i, rule.Name())
		}
		if rule.Index() != i {
			t.Errorf("expect rule %s with index %d, got %d", r, i, rule.Index())
		}
		if f.function.stmt.Body[i] != rule.expr {
			t.Errorf("underlying syntax tree of rule %s not sorted", r)
		}
	}

	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
load("@bazel_gazelle//:deps.bzl", "go_repository")

def repos():
    a_rule(
        name = "name1",
    )
    go_repository(
        name = "com_github_andybalholm_cascadia",
    )

    go_repository(
        name = "com_github_bazelbuild_bazel_gazelle",
    )
    go_repository(
        name = "com_github_bazelbuild_buildtools",
    )
    go_repository(
        name = "com_github_bazelbuild_rules_go",
    )
    some_other_call_rule()
`)

	if got != want {
		t.Errorf("got:%s\nwant:%s", got, want)
	}
}

func TestCheckFile(t *testing.T) {
	f := File{Rules: []*Rule{
		NewRule("go_repository", "com_google_cloud_go_pubsub"),
		NewRule("go_repository", "com_google_cloud_go_pubsub"),
	}}
	err := checkFile(&f)
	if err == nil {
		t.Errorf("muliple rules with the same name should not be tolerated")
	}

	f = File{Rules: []*Rule{
		NewRule("go_rules_dependencies", ""),
		NewRule("go_register_toolchains", ""),
	}}
	err = checkFile(&f)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAttributeComment(t *testing.T) {
	f, err := LoadData(filepath.Join("old", "BUILD.bazel"), "", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := NewRule("a_rule", "name1")
	r.SetAttr("deps", []string{"foo", "bar", "baz"})
	r.SetAttr("hdrs", []string{"foo", "bar", "baz"})
	hdrComments := r.AttrComments("hdrs")
	hdrComments.Before = append(hdrComments.Before, bzl.Comment{
		Token: "# do not sort",
	})

	r.Insert(f)

	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
a_rule(
    name = "name1",
    # do not sort
    hdrs = [
        "foo",
        "bar",
        "baz",
    ],
    deps = [
        "bar",
        "baz",
        "foo",
    ],
)
`)

	if got != want {
		t.Errorf("got:%s\nwant:%s", got, want)
	}
}

func TestSimpleArgument(t *testing.T) {
	f := EmptyFile("foo", "bar")

	r := NewRule("export_files", "")
	r.AddArg(&bzl.CallExpr{
		X: &bzl.Ident{Name: "glob"},
		List: []bzl.Expr{
			&bzl.ListExpr{
				List: []bzl.Expr{
					&bzl.StringExpr{Value: "**"},
				},
			},
		},
	})

	r.Insert(f)
	f.Sync()

	got := strings.TrimSpace(string(bzl.FormatWithoutRewriting(f.File)))
	want := strings.TrimSpace(`
export_files(glob(["**"]))
`)

	if got != want {
		t.Errorf("got:%s\nwant:%s", got, want)
	}
}

func TestAttributeValueSorting(t *testing.T) {
	f := EmptyFile("foo", "bar")

	r := NewRule("a_rule", "")
	r.SetAttr("deps", []string{"foo", "bar", "baz"})
	r.SetAttr("srcs", UnsortedStrings{"foo", "bar", "baz"})
	r.SetAttr("hdrs", []string{"foo", "bar", "baz"})

	r.Insert(f)
	f.Sync()

	got := strings.TrimSpace(string(bzl.FormatWithoutRewriting(f.File)))
	want := strings.TrimSpace(`
a_rule(
    srcs = [
        "foo",
        "bar",
        "baz",
    ],
    hdrs = [
        "foo",
        "bar",
        "baz",
    ],
    deps = [
        "bar",
        "baz",
        "foo",
    ],
)
`)

	if got != want {
		t.Errorf("got:%s\nwant:%s", got, want)
	}
}

func TestAttributeValueSortingOverride(t *testing.T) {
	f := EmptyFile("foo", "bar")

	r := NewRule("a_rule", "")
	r.SetAttr("deps", []string{"foo", "bar", "baz"})
	r.SetAttr("srcs", UnsortedStrings{"foo", "bar", "baz"})
	r.SetAttr("hdrs", []string{"foo", "bar", "baz"})
	r.SetSortedAttrs([]string{"srcs", "hdrs"})

	r.Insert(f)
	f.Sync()

	got := strings.TrimSpace(string(bzl.FormatWithoutRewriting(f.File)))
	want := strings.TrimSpace(`
a_rule(
    srcs = [
        "foo",
        "bar",
        "baz",
    ],
    hdrs = [
        "bar",
        "baz",
        "foo",
    ],
    deps = [
        "foo",
        "bar",
        "baz",
    ],
)
`)

	if got != want {
		t.Errorf("got:%s\nwant:%s", got, want)
	}

	if !reflect.DeepEqual(r.SortedAttrs(), []string{"srcs", "hdrs"}) {
		t.Errorf("Unexpected r.SortedAttrs(): %v", r.SortedAttrs())
	}
}

func TestAttributeBool(t *testing.T) {
	for _, tc := range []struct {
		desc, src string
		want      bool
	}{
		{
			desc: "missing attribute returns false",
			src:  `my_rule(name = "my_name")`,
			want: false,
		}, {
			desc: "attribute is true",
			src:  `my_rule(name = "my_name", bool_attr = True)`,
			want: true,
		}, {
			desc: "attribute is false",
			src:  `my_rule(name = "my_name", bool_attr = False)`,
			want: false,
		}, {
			desc: "attribute is not a boolean literal returns false",
			src:  `my_rule(name = "my_name", bool_attr = "foo")`,
			want: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := LoadData(filepath.Join("old", "BUILD.bazel"), "", []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Rules) != 1 {
				t.Fatalf("expected 1 rule, got %d", len(f.Rules))
			}
			r := f.Rules[0]
			got := r.AttrBool("bool_attr")
			if got != tc.want {
				t.Errorf("got %v; want %v", got, tc.want)
			}
		})
	}
}
