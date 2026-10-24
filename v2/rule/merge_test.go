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

package rule_test

import (
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

func TestMergeRules(t *testing.T) {
	t.Run("private attributes are merged", func(t *testing.T) {
		src := rule.NewRule("go_library", "go_default_library")
		privateAttrKey := "_my_private_attr"
		privateAttrVal := "private_value"
		src.SetPrivateAttr(privateAttrKey, privateAttrVal)
		dst := rule.NewRule("go_library", "go_default_library")
		rule.MergeRules(src, dst, map[string]bool{}, "")
		if dst.PrivateAttr(privateAttrKey).(string) != privateAttrVal {
			t.Fatalf("private attributes are merged: got %v; want %s",
				dst.PrivateAttr(privateAttrKey), privateAttrVal)
		}
	})
}

func TestMergeRules_WithSortedStringAttr(t *testing.T) {
	t.Run("sorted string attributes are merged to empty rule", func(t *testing.T) {
		src := rule.NewRule("go_library", "go_default_library")
		sortedStringAttrKey := "deps"
		sortedStringAttrVal := rule.SortedStrings{"@qux", "//foo:bar", "//foo:baz"}
		src.SetAttr(sortedStringAttrKey, sortedStringAttrVal)
		dst := rule.NewRule("go_library", "go_default_library")
		rule.MergeRules(src, dst, map[string]bool{}, "")

		valExpr, ok := dst.Attr(sortedStringAttrKey).(*bzl.ListExpr)
		if !ok {
			t.Fatalf("sorted string attributes invalid: got %v; want *bzl.ListExpr",
				dst.Attr(sortedStringAttrKey))
		}

		expected := []string{"//foo:bar", "//foo:baz", "@qux"}
		for i, v := range valExpr.List {
			if v.(*bzl.StringExpr).Value != expected[i] {
				t.Fatalf("sorted string attributes are merged: got %v; want %v",
					v.(*bzl.StringExpr).Value, expected[i])
			}
		}
	})

	t.Run("sorted string attributes are merged to non-empty rule", func(t *testing.T) {
		src := rule.NewRule("go_library", "go_default_library")
		sortedStringAttrKey := "deps"
		sortedStringAttrVal := rule.SortedStrings{"@qux", "//foo:bar", "//foo:baz"}
		src.SetAttr(sortedStringAttrKey, sortedStringAttrVal)
		dst := rule.NewRule("go_library", "go_default_library")
		dst.SetAttr(sortedStringAttrKey, rule.SortedStrings{"@qux", "//foo:bar", "//bacon:eggs"})
		rule.MergeRules(src, dst, map[string]bool{"deps": true}, "")

		valExpr, ok := dst.Attr(sortedStringAttrKey).(*bzl.ListExpr)
		if !ok {
			t.Fatalf("sorted string attributes are merged: got %v; want *bzl.ListExpr",
				dst.Attr(sortedStringAttrKey))
		}

		expected := []string{"//foo:bar", "//foo:baz", "@qux"}
		for i, v := range valExpr.List {
			if v.(*bzl.StringExpr).Value != expected[i] {
				t.Fatalf("sorted string attributes are merged: got %v; want %v",
					v.(*bzl.StringExpr).Value, expected[i])
			}
		}
	})
	t.Run("delete existing sorted strings", func(t *testing.T) {
		src := rule.NewRule("go_library", "go_default_library")
		sortedStringAttrKey := "deps"
		dst := rule.NewRule("go_library", "go_default_library")
		sortedStringAttrVal := rule.SortedStrings{"@qux", "//foo:bar", "//foo:baz"}
		dst.SetAttr(sortedStringAttrKey, sortedStringAttrVal)
		rule.MergeRules(src, dst, map[string]bool{"deps": true}, "")

		if dst.Attr(sortedStringAttrKey) != nil {
			t.Fatalf("delete existing sorted strings: got %v; want nil",
				dst.Attr(sortedStringAttrKey))
		}
	})
}

func TestMergeRules_WithUnsortedStringAttr(t *testing.T) {
	t.Run("unsorted string attributes are merged to empty rule", func(t *testing.T) {
		src := rule.NewRule("go_library", "go_default_library")
		sortedStringAttrKey := "deps"
		sortedStringAttrVal := rule.UnsortedStrings{"@qux", "//foo:bar", "//foo:baz"}
		src.SetAttr(sortedStringAttrKey, sortedStringAttrVal)
		dst := rule.NewRule("go_library", "go_default_library")
		rule.MergeRules(src, dst, map[string]bool{}, "")

		valExpr, ok := dst.Attr(sortedStringAttrKey).(*bzl.ListExpr)
		if !ok {
			t.Fatalf("sorted string attributes invalid: got %v; want *bzl.ListExpr",
				dst.Attr(sortedStringAttrKey))
		}

		expected := []string{"@qux", "//foo:bar", "//foo:baz"}
		for i, v := range valExpr.List {
			if v.(*bzl.StringExpr).Value != expected[i] {
				t.Fatalf("unsorted string attributes are merged: got %v; want %v",
					v.(*bzl.StringExpr).Value, expected[i])
			}
		}
	})

	t.Run("unsorted string attributes are merged to non-empty rule", func(t *testing.T) {
		src := rule.NewRule("go_library", "go_default_library")
		sortedStringAttrKey := "deps"
		sortedStringAttrVal := rule.UnsortedStrings{"@qux", "//foo:bar", "//foo:baz"}
		src.SetAttr(sortedStringAttrKey, sortedStringAttrVal)
		dst := rule.NewRule("go_library", "go_default_library")
		dst.SetAttr(sortedStringAttrKey, rule.UnsortedStrings{"@qux", "//foo:bar", "//bacon:eggs"})
		rule.MergeRules(src, dst, map[string]bool{"deps": true}, "")

		valExpr, ok := dst.Attr(sortedStringAttrKey).(*bzl.ListExpr)
		if !ok {
			t.Fatalf("unsorted string attributes are merged: got %v; want *bzl.ListExpr",
				dst.Attr(sortedStringAttrKey))
		}

		expected := []string{"@qux", "//foo:bar", "//foo:baz"}
		for i, v := range valExpr.List {
			if v.(*bzl.StringExpr).Value != expected[i] {
				t.Fatalf("unsorted string attributes are merged: got %v; want %v",
					v.(*bzl.StringExpr).Value, expected[i])
			}
		}
	})
}

func TestMergeRules_DictWithExplicitEmptyList(t *testing.T) {
	const want = `{
    "@platforms//os:linux": [],
    "//conditions:default": ["//lib"],
}`

	srcDict := &bzl.DictExpr{List: []*bzl.KeyValueExpr{
		{
			Key:   &bzl.StringExpr{Value: "@platforms//os:linux"},
			Value: &bzl.ListExpr{List: []bzl.Expr{}},
		},
		{
			Key:   &bzl.StringExpr{Value: "//conditions:default"},
			Value: &bzl.ListExpr{List: []bzl.Expr{&bzl.StringExpr{Value: "//lib"}}},
		},
	}}

	for _, tc := range []struct {
		desc    string
		dstDict *bzl.DictExpr
	}{
		{
			desc: "explicit_empty_list_preserved",
			dstDict: &bzl.DictExpr{List: []*bzl.KeyValueExpr{
				{
					Key:   &bzl.StringExpr{Value: "@platforms//os:linux"},
					Value: &bzl.ListExpr{List: []bzl.Expr{}},
				},
			}},
		},
		{
			desc:    "dst_nil_returns_src",
			dstDict: nil,
		},
		{
			desc: "dst_default_only_merged",
			dstDict: &bzl.DictExpr{List: []*bzl.KeyValueExpr{
				{
					Key:   &bzl.StringExpr{Value: "//conditions:default"},
					Value: &bzl.ListExpr{List: []bzl.Expr{&bzl.StringExpr{Value: "//other"}}},
				},
			}},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			merged, err := rule.MergeDict(srcDict, tc.dstDict)
			if err != nil {
				t.Fatalf("MergeDict: %v", err)
			}
			if merged == nil {
				t.Fatalf("MergeDict: got nil; want non-nil dict")
			}
			got := bzl.FormatString(merged)
			if got != want {
				t.Errorf("MergeDict: got %s; want %s", got, want)
			}
		})
	}
}

func TestMergeRules_SelectStringListValueWithExplicitEmptyList(t *testing.T) {
	const (
		ruleKind = "foo"
		ruleName = "bar"
		want     = `select({
    "@platforms//os:linux": [],
    "//conditions:default": ["//lib"],
})`
	)
	mergeable := map[string]bool{"deps": true}
	srcRule := rule.NewRule(ruleKind, ruleName)
	srcRule.SetAttr("deps", rule.SelectStringListValue{
		"@platforms//os:linux": {},
		"//conditions:default": {"//lib"},
	})

	for _, tc := range []struct {
		desc    string
		dstDeps rule.SelectStringListValue
	}{
		{
			desc: "explicit_empty_list_preserved",
			dstDeps: rule.SelectStringListValue{
				"@platforms//os:linux": {},
			},
		},
		{
			desc:    "dst_nil_returns_src",
			dstDeps: nil,
		},
		{
			desc: "dst_default_only_merged",
			dstDeps: rule.SelectStringListValue{
				"//conditions:default": {"//other"},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dstRule := rule.NewRule(ruleKind, ruleName)
			dstRule.SetAttr("deps", tc.dstDeps)
			rule.MergeRules(srcRule, dstRule, mergeable, "")
			got := bzl.FormatString(dstRule.Attr("deps"))
			if got != want {
				t.Errorf("MergeRules: got %s; want %s", got, want)
			}
		})
	}
}

// cgoSelectExpr returns a select that maps each key to value and the default
// case to the opposite value, like the cgo attribute that Gazelle generates.
func cgoSelectExpr(value bool, keys ...string) bzl.Expr {
	ident := func(b bool) bzl.Expr {
		if b {
			return &bzl.Ident{Name: "True"}
		}
		return &bzl.Ident{Name: "False"}
	}
	dict := &bzl.DictExpr{ForceMultiLine: true}
	for _, key := range keys {
		dict.List = append(dict.List, &bzl.KeyValueExpr{
			Key:   &bzl.StringExpr{Value: key},
			Value: ident(value),
		})
	}
	dict.List = append(dict.List, &bzl.KeyValueExpr{
		Key:   &bzl.StringExpr{Value: "//conditions:default"},
		Value: ident(!value),
	})
	return &bzl.CallExpr{X: &bzl.Ident{Name: "select"}, List: []bzl.Expr{dict}}
}

const (
	darwinKey  = "@io_bazel_rules_go//go/platform:darwin"
	linuxKey   = "@io_bazel_rules_go//go/platform:linux"
	windowsKey = "@io_bazel_rules_go//go/platform:windows"
)

func TestMergeRules_ScalarSelect(t *testing.T) {
	for _, tc := range []struct {
		desc string
		// old is an existing build file with one rule.
		old string
		// attr is the attribute to check. It defaults to "cgo".
		attr string
		// src is the generated value of attr, or nil if it is not generated.
		src bzl.Expr
		// want is the merged value of attr, or "" if merge deletes it.
		want string
	}{
		{
			desc: "delete_when_src_omits_and_not_kept",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    }),
)
`,
		},
		{
			// Without a trailing comma, the parser attaches the comment to
			// the assignment, which keeps the attribute.
			desc: "preserve_assignment_keep_without_trailing_comma",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    })  # keep
)
`,
			want: `select({
    "@io_bazel_rules_go//go/platform:darwin": True,
    "//conditions:default": False,
})`,
		},
		{
			desc: "preserve_kept_entry_inside_select",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "@io_bazel_rules_go//go/platform:windows": True,  # keep
        "//conditions:default": False,
    }),
)
`,
			src: cgoSelectExpr(true, linuxKey),
			want: `select({
    "@io_bazel_rules_go//go/platform:linux": True,
    "@io_bazel_rules_go//go/platform:windows": True,  # keep
    "//conditions:default": False,
})`,
		},
		{
			desc: "preserve_kept_entry_when_src_omits",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "@io_bazel_rules_go//go/platform:windows": True,  # keep
        "//conditions:default": False,
    }),
)
`,
			want: `select({
    "@io_bazel_rules_go//go/platform:windows": True,  # keep
    "//conditions:default": False,
})`,
		},
		{
			desc: "keep_on_select_call_preserves_select_when_src_omits",
			old: `go_library(
    name = "foo",
    cgo = select(  # keep
        {
            "@io_bazel_rules_go//go/platform:darwin": True,
            "//conditions:default": False,
        },
    ),
)
`,
			want: `select(
    # keep
    {
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    },
)`,
		},
		{
			desc: "keep_on_select_call_preserves_select",
			old: `go_library(
    name = "foo",
    cgo = select(  # keep
        {
            "@io_bazel_rules_go//go/platform:darwin": True,
            "//conditions:default": False,
        },
    ),
)
`,
			src: cgoSelectExpr(true, linuxKey),
			want: `select(
    # keep
    {
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    },
)`,
		},
		{
			desc: "keep_on_dict_line_preserves_select_when_src_omits",
			old: `go_library(
    name = "foo",
    cgo = select({  # keep
        "@io_bazel_rules_go//go/platform:darwin": True,
        "@io_bazel_rules_go//go/platform:windows": True,
        "//conditions:default": False,
    }),
)
`,
			want: `select({
    # keep
    "@io_bazel_rules_go//go/platform:darwin": True,
    "@io_bazel_rules_go//go/platform:windows": True,
    "//conditions:default": False,
})`,
		},
		{
			desc: "keep_on_dict_line_preserves_select",
			old: `go_library(
    name = "foo",
    cgo = select({  # keep
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    }),
)
`,
			src: cgoSelectExpr(true, linuxKey),
			want: `select({
    # keep
    "@io_bazel_rules_go//go/platform:darwin": True,
    "//conditions:default": False,
})`,
		},
		{
			// A keep comment on its own line before the first case applies
			// to that case only.
			desc: "keep_before_first_case_preserves_that_case",
			old: `go_library(
    name = "foo",
    cgo = select({
        # keep
        "@io_bazel_rules_go//go/platform:darwin": True,
        "@io_bazel_rules_go//go/platform:windows": True,
        "//conditions:default": False,
    }),
)
`,
			src: cgoSelectExpr(true, linuxKey),
			want: `select({
    # keep
    "@io_bazel_rules_go//go/platform:darwin": True,
    "@io_bazel_rules_go//go/platform:linux": True,
    "//conditions:default": False,
})`,
		},
		{
			// Merge does not manage selects with keys that are not platform
			// labels. Without a generated value, it keeps them, as before.
			desc: "custom_key_select_unchanged_when_src_omits",
			old: `go_binary(
    name = "bin",
    pgoprofile = select({
        ":opt": "opt.pgo",
        "//conditions:default": None,
    }),
)
`,
			attr: "pgoprofile",
			want: `select({
    ":opt": "opt.pgo",
    "//conditions:default": None,
})`,
		},
		{
			desc: "custom_key_select_with_keep_preserved",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@custom//:cgo": True,  # keep
        "@io_bazel_rules_go//go/platform:windows": True,
        "//conditions:default": False,
    }),
)
`,
			src: cgoSelectExpr(true, darwinKey),
			want: `select({
    "@custom//:cgo": True,  # keep
    "@io_bazel_rules_go//go/platform:windows": True,
    "//conditions:default": False,
})`,
		},
		{
			desc: "custom_key_select_without_keep_replaced",
			old: `go_library(
    name = "foo",
    cgo = select({
        "//:cgo_on": True,
        "//conditions:default": False,
    }),
)
`,
			src: cgoSelectExpr(true, darwinKey),
			want: `select({
    "@io_bazel_rules_go//go/platform:darwin": True,
    "//conditions:default": False,
})`,
		},
		{
			desc: "inverted_select_replaces_positive_select",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    }),
)
`,
			src: cgoSelectExpr(false, windowsKey),
			want: `select({
    "@io_bazel_rules_go//go/platform:windows": False,
    "//conditions:default": True,
})`,
		},
		{
			desc: "positive_select_replaces_inverted_select",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:windows": False,
        "//conditions:default": True,
    }),
)
`,
			src: cgoSelectExpr(true, darwinKey),
			want: `select({
    "@io_bazel_rules_go//go/platform:darwin": True,
    "//conditions:default": False,
})`,
		},
		{
			desc: "kept_entry_survives_change_to_inverted_select",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "@io_bazel_rules_go//go/platform:plan9": True,  # keep
        "//conditions:default": False,
    }),
)
`,
			src: cgoSelectExpr(false, windowsKey),
			want: `select({
    "@io_bazel_rules_go//go/platform:plan9": True,  # keep
    "@io_bazel_rules_go//go/platform:windows": False,
    "//conditions:default": True,
})`,
		},
		{
			desc: "kept_entry_survives_change_to_scalar",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "@io_bazel_rules_go//go/platform:windows": False,  # keep
        "//conditions:default": False,
    }),
)
`,
			src: &bzl.Ident{Name: "True"},
			want: `select({
    "@io_bazel_rules_go//go/platform:windows": False,  # keep
    "//conditions:default": True,
})`,
		},
		{
			desc: "unkept_select_replaced_by_scalar",
			old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    }),
)
`,
			src:  &bzl.Ident{Name: "True"},
			want: `True`,
		},
		{
			desc: "keep_on_select_call_preserves_select_when_src_is_scalar",
			old: `go_library(
    name = "foo",
    cgo = select(  # keep
        {
            "@io_bazel_rules_go//go/platform:darwin": True,
            "//conditions:default": False,
        },
    ),
)
`,
			src: &bzl.Ident{Name: "True"},
			want: `select(
    # keep
    {
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    },
)`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := rule.LoadData("BUILD.bazel", "", []byte(tc.old))
			if err != nil {
				t.Fatalf("LoadData: %v", err)
			}
			attr := tc.attr
			if attr == "" {
				attr = "cgo"
			}
			dstRule := f.Rules[0]
			srcRule := rule.NewRule(dstRule.Kind(), dstRule.Name())
			if tc.src != nil {
				srcRule.SetAttr(attr, tc.src)
			}
			rule.MergeRules(srcRule, dstRule, map[string]bool{attr: true}, "BUILD.bazel")
			got := ""
			if e := dstRule.Attr(attr); e != nil {
				got = bzl.FormatString(e)
			}
			if got != tc.want {
				t.Errorf("merged %s:\ngot:\n%s\nwant:\n%s", attr, got, tc.want)
			}
		})
	}

	// The parser never attaches a keep comment to the value of an attribute,
	// so this test adds one directly. Merge used to delete such attributes.
	t.Run("preserve_keep_on_attribute_value", func(t *testing.T) {
		for _, src := range []bzl.Expr{nil, &bzl.Ident{Name: "True"}} {
			f, err := rule.LoadData("BUILD.bazel", "", []byte(`go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    }),
    copts = ["-DFOO"],
)
`))
			if err != nil {
				t.Fatalf("LoadData: %v", err)
			}
			dstRule := f.Rules[0]
			for _, attr := range []string{"cgo", "copts"} {
				value := dstRule.Attr(attr)
				value.Comment().Suffix = append(value.Comment().Suffix, bzl.Comment{Token: "# keep"})
			}
			srcRule := rule.NewRule("go_library", "foo")
			if src != nil {
				srcRule.SetAttr("cgo", src)
			}
			rule.MergeRules(srcRule, dstRule, map[string]bool{"cgo": true, "copts": true}, "BUILD.bazel")
			if got := dstRule.Attr("cgo"); got == nil || !rule.ShouldKeep(got) || !isSelectExpr(got) {
				t.Errorf("src %v: kept cgo select changed to %v", src, got)
			}
			if got := dstRule.Attr("copts"); got == nil || !rule.ShouldKeep(got) {
				t.Errorf("src %v: kept copts changed to %v", src, got)
			}
		}
	})

	t.Run("scalar_to_select_and_back", func(t *testing.T) {
		f, err := rule.LoadData("BUILD.bazel", "", []byte(`go_library(
    name = "foo",
    cgo = True,
)
`))
		if err != nil {
			t.Fatalf("LoadData: %v", err)
		}
		dstRule := f.Rules[0]
		srcSelect := rule.NewRule("go_library", "foo")
		srcSelect.SetAttr("cgo", cgoSelectExpr(true, darwinKey))
		rule.MergeRules(srcSelect, dstRule, map[string]bool{"cgo": true}, "BUILD.bazel")
		wantSelect := `select({
    "@io_bazel_rules_go//go/platform:darwin": True,
    "//conditions:default": False,
})`
		if got := bzl.FormatString(dstRule.Attr("cgo")); got != wantSelect {
			t.Fatalf("True -> select got:\n%s\nwant:\n%s", got, wantSelect)
		}

		srcTrue := rule.NewRule("go_library", "foo")
		srcTrue.SetAttr("cgo", true)
		rule.MergeRules(srcTrue, dstRule, map[string]bool{"cgo": true}, "BUILD.bazel")
		if got := bzl.FormatString(dstRule.Attr("cgo")); got != "True" {
			t.Fatalf("select -> True got %s; want True", got)
		}
	})

	t.Run("remove_noop_keep_comments", func(t *testing.T) {
		orig := rule.RemoveNoopKeepComments
		rule.RemoveNoopKeepComments = true
		t.Cleanup(func() { rule.RemoveNoopKeepComments = orig })

		for _, tc := range []struct {
			desc string
			old  string
			src  bzl.Expr
			want string
		}{
			{
				desc: "kept_entry_matching_generated_entry_loses_keep",
				old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,  # keep
        "//conditions:default": False,
    }),
)
`,
				src: cgoSelectExpr(true, darwinKey),
				want: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    }),
)
`,
			},
			{
				desc: "kept_assignment_matching_generated_select_loses_keep",
				old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    })  # keep
)
`,
				src: cgoSelectExpr(true, darwinKey),
				want: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    }),
)
`,
			},
			{
				desc: "kept_assignment_differing_from_generated_select_keeps_keep",
				old: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    })  # keep
)
`,
				src: cgoSelectExpr(true, linuxKey),
				want: `go_library(
    name = "foo",
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "//conditions:default": False,
    }),  # keep
)
`,
			},
		} {
			t.Run(tc.desc, func(t *testing.T) {
				f, err := rule.LoadData("BUILD.bazel", "", []byte(tc.old))
				if err != nil {
					t.Fatalf("LoadData: %v", err)
				}
				dstRule := f.Rules[0]
				srcRule := rule.NewRule("go_library", "foo")
				srcRule.SetAttr("cgo", tc.src)
				rule.MergeRules(srcRule, dstRule, map[string]bool{"cgo": true}, "BUILD.bazel")
				f.Sync()
				if got := string(f.Format()); got != tc.want {
					t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
				}
			})
		}
	})
}

func isSelectExpr(e bzl.Expr) bool {
	call, ok := e.(*bzl.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.X.(*bzl.Ident)
	return ok && ident.Name == "select"
}

// TestMergeRules_NoStringInListAndSelect checks that a merged expression
// never has a string in both the generic list and a select. Bazel rejects
// such duplicates on the selected platform.
func TestMergeRules_NoStringInListAndSelect(t *testing.T) {
	for _, tc := range []struct {
		desc, old, attr string
		src             interface{}
		want            string
	}{
		{
			desc: "kept_list_entry_and_generated_select",
			old: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",  # keep
        "pure.go",
    ],
)
`,
			attr: "srcs",
			src: rule.PlatformStrings{
				Generic: []string{"pure.go"},
				OS:      map[string][]string{darwinKey: {"cgo_darwin.go"}},
			},
			want: `[
    "cgo_darwin.go",  # keep
    "pure.go",
]`,
		},
		{
			desc: "kept_select_entry_and_generated_list",
			old: `go_library(
    name = "foo",
    srcs = [
        "pure.go",
    ] + select({
        "@io_bazel_rules_go//go/platform:darwin": [
            "cgo_darwin.go",
            "helper_darwin.c",  # keep
        ],
        "//conditions:default": [],
    }),
)
`,
			attr: "srcs",
			src:  []string{"cgo_all.go", "cgo_darwin.go", "helper_darwin.c", "pure.go"},
			want: `[
    "pure.go",
    "cgo_all.go",
    "cgo_darwin.go",
    "helper_darwin.c",
]`,
		},
		{
			desc: "other_select_entries_stay",
			old: `go_library(
    name = "foo",
    deps = [
        "//a",  # keep
    ] + select({
        "@io_bazel_rules_go//go/platform:linux": [
            "//a",  # keep
            "//b",  # keep
        ],
        "//conditions:default": [],
    }),
)
`,
			attr: "deps",
			src:  []string{"//c"},
			want: `[
    "//a",  # keep
    "//c",
] + select({
    "@io_bazel_rules_go//go/platform:linux": [
        "//b",  # keep
    ],
    "//conditions:default": [],
})`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := rule.LoadData("BUILD.bazel", "", []byte(tc.old))
			if err != nil {
				t.Fatalf("LoadData: %v", err)
			}
			dstRule := f.Rules[0]
			srcRule := rule.NewRule("go_library", "foo")
			srcRule.SetAttr(tc.attr, tc.src)
			rule.MergeRules(srcRule, dstRule, map[string]bool{tc.attr: true}, "BUILD.bazel")
			got := bzl.FormatString(dstRule.Attr(tc.attr))
			if got != tc.want {
				t.Errorf("merged %s:\ngot:\n%s\nwant:\n%s", tc.attr, got, tc.want)
			}
		})
	}
}
