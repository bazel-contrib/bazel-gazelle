package mapkind

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func TestResult_ApplyOnLoads(t *testing.T) {
	loads := []rule.LoadInfo{
		{
			Name: "//my:defs.bzl",
			Symbols: []string{
				"my_rule",
				"my_second_rule",
			},
		},
	}

	t.Run("no_mapping", func(t *testing.T) {
		mr := MappedResult{}

		// Copy to ensure ApplyOnLoads don't modify the given data.
		expectedLoads := make([]rule.LoadInfo, len(loads))
		copy(expectedLoads, loads)

		result := mr.ApplyOnLoads(loads)

		if diff := cmp.Diff(expectedLoads, result); diff != "" {
			t.Errorf("(-want, +got):\n%s", diff)
		}
	})

	t.Run("with_mapping", func(t *testing.T) {
		mr := MappedResult{
			MappedKinds: []config.MappedKind{
				{
					FromKind: "my_rule",
					KindName: "other_rule",
					KindLoad: "//other:defs.bzl",
				},
			},
		}

		expectedLoads := []rule.LoadInfo{
			{
				Name: "//my:defs.bzl",
				Symbols: []string{
					"my_rule",
					"my_second_rule",
				},
			},
			{
				Name: "//other:defs.bzl",
				Symbols: []string{
					"other_rule",
				},
			},
		}

		result := mr.ApplyOnLoads(loads)

		if diff := cmp.Diff(expectedLoads, result); diff != "" {
			t.Errorf("(-want, +got):\n%s", diff)
		}
	})

	t.Run("with_recursive_mapping", func(t *testing.T) {
		mr := MappedResult{
			MappedKinds: []config.MappedKind{
				{
					FromKind: "my_rule",
					KindName: "other_rule",
					KindLoad: "//other:defs.bzl",
				},
				{
					FromKind: "other_rule",
					KindName: "other_other_rule",
					KindLoad: "//other/other:defs.bzl",
				},
			},
		}

		expectedLoads := []rule.LoadInfo{
			{
				Name: "//my:defs.bzl",
				Symbols: []string{
					"my_rule",
					"my_second_rule",
				},
			},
			{
				Name: "//other:defs.bzl",
				Symbols: []string{
					"other_rule",
				},
			},
			{
				Name: "//other/other:defs.bzl",
				Symbols: []string{
					"other_other_rule",
				},
			},
		}

		result := mr.ApplyOnLoads(loads)

		if diff := cmp.Diff(expectedLoads, result); diff != "" {
			t.Errorf("(-want, +got):\n%s", diff)
		}
	})
}
