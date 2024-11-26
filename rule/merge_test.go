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
	"reflect"
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
		rule.MergeRules(src, dst, map[string]bool{"deps": true}, "")

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
		rule.MergeRules(src, dst, map[string]bool{"deps": true}, "")

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

func TestMergeRules_WithUnmanagedAttr(t *testing.T) {
	t.Run("unknown string attributes overwrite attributes in non-empty rule", func(t *testing.T) {
		src := rule.NewRule("go_library", "go_default_library")
		unknownStringAttrKey := "unknown_attr"
		overwrittenAttrVal := rule.UnsortedStrings{"@qux", "//foo:bar", "//foo:baz"}
		src.SetAttr(unknownStringAttrKey, "foobar")
		dst := rule.NewRule("go_library", "go_default_library")
		dst.SetAttr(unknownStringAttrKey, overwrittenAttrVal)
		rule.MergeRules(src, dst, map[string]bool{}, "")

		valExpr, ok := dst.Attr(unknownStringAttrKey).(*bzl.StringExpr)
		if !ok {
			t.Fatalf("unknown attributes (%q) are overwritten: got %v; want *bzl.StringExpr",
				unknownStringAttrKey, reflect.TypeOf(dst.Attr(unknownStringAttrKey)))
		}

		expected := "foobar"
		if valExpr.Value != expected {
			t.Fatalf("unknown attributes (%q) are overwritten: got %v; want %v",
				unknownStringAttrKey, valExpr.Value, expected)
		}
	})

	t.Run("known string attributes do NOT overwrite attributes in non-empty rule", func(t *testing.T) {
		src := rule.NewRule("go_library", "go_default_library")
		unknownStringAttrKey := "unknown_attr"
		knownStringAttrKey := "knownAttrName"
		overwrittenAttrVal := "original"
		src.SetAttr(unknownStringAttrKey, "foobar")
		src.SetAttr(knownStringAttrKey, "foobar")
		dst := rule.NewRule("go_library", "go_default_library")
		dst.SetAttr(unknownStringAttrKey, overwrittenAttrVal)
		dst.SetAttr(knownStringAttrKey, overwrittenAttrVal)
		rule.MergeRules(src, dst, map[string]bool{knownStringAttrKey: false}, "")

		// The unknown + overwritten attribute
		unknownValExpr, ok := dst.Attr(unknownStringAttrKey).(*bzl.StringExpr)
		if !ok {
			t.Fatalf("unknown attributes (%q) are overwritten: got %v; want *bzl.StringExpr",
				unknownStringAttrKey, reflect.TypeOf(dst.Attr(unknownStringAttrKey)))
		}
		expected := "foobar"
		if unknownValExpr.Value != expected {
			t.Fatalf("unknown attributes (%q) are overwritten: got %v; want %v",
				unknownStringAttrKey, unknownValExpr.Value, expected)
		}

		// The known but non-mergeable attributes
		knownValExpr, ok := dst.Attr(knownStringAttrKey).(*bzl.StringExpr)
		if !ok {
			t.Fatalf("known attributes (%q) are NOT overwritten: got %v; want *bzl.StringExpr",
				knownStringAttrKey, reflect.TypeOf(dst.Attr(knownStringAttrKey)))
		}
		if knownValExpr.Value != overwrittenAttrVal {
			t.Fatalf("known attributes (%q) are NOT overwritten: got %v; want %v",
				knownStringAttrKey, knownValExpr.Value, overwrittenAttrVal)
		}
	})
}
