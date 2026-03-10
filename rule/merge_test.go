/* Copyright 2021 The Bazel Authors. All rights reserved.

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

	"github.com/bazelbuild/bazel-gazelle/rule"
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

func TestMergeDict_SelectWithExplicitEmptyList(t *testing.T) {
	// select({"@platforms//os:linux": [], "//conditions:default": ["//lib"]})
	// Empty list for a select case must not be dropped when merging.
	srcDict := &bzl.DictExpr{
		List: []*bzl.KeyValueExpr{
			{
				Key:   &bzl.StringExpr{Value: "@platforms//os:linux"},
				Value: &bzl.ListExpr{List: []bzl.Expr{}},
			},
			{
				Key: &bzl.StringExpr{Value: "//conditions:default"},
				Value: &bzl.ListExpr{
					List: []bzl.Expr{&bzl.StringExpr{Value: "//lib"}},
				},
			},
		},
	}
	// dst has the same key with empty list so MergeList([], []) returns nil;
	// MergeDict must still keep the explicit empty list for the linux case.
	dstDict := &bzl.DictExpr{
		List: []*bzl.KeyValueExpr{
			{
				Key:   &bzl.StringExpr{Value: "@platforms//os:linux"},
				Value: &bzl.ListExpr{List: []bzl.Expr{}},
			},
		},
	}

	merged, err := rule.MergeDict(srcDict, dstDict)
	if err != nil {
		t.Fatalf("MergeDict: %v", err)
	}
	if merged == nil {
		t.Fatal("MergeDict: got nil; want non-nil dict with both cases preserved")
	}

	// Find "@platforms//os:linux" and ensure it has an explicit empty list.
	var gotLinuxList *bzl.ListExpr
	for _, kv := range merged.List {
		if s, ok := kv.Key.(*bzl.StringExpr); ok && s.Value == "@platforms//os:linux" {
			if l, ok := kv.Value.(*bzl.ListExpr); ok {
				gotLinuxList = l
				break
			}
		}
	}
	if gotLinuxList == nil {
		t.Fatal("MergeDict: result missing \"@platforms//os:linux\" case")
	}
	if len(gotLinuxList.List) != 0 {
		t.Errorf("MergeDict: \"@platforms//os:linux\" should be empty list; got len=%d", len(gotLinuxList.List))
	}
}
