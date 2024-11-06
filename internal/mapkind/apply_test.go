package mapkind

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

var testDataBuildFile = []byte(`load("//my:defs.bzl", "my_rule")
load("@other_rules//:defs.bzl", "something")

maybe(
    my_rule,
    something,
    name = "maybe",
    first_arg = 1,
    second_arg = "x",
)

my_rule(
    name = "always",
    first_arg = 2,
    second_arg = "y",
)
`)

func TestApplyOnRules(t *testing.T) {
	f, err := rule.LoadData("test/file.BUILD.bazel", "", testDataBuildFile)
	if err != nil {
		t.Fatalf("can't load nominal build file: %v", err)
	}

	knownKinds := map[string]rule.KindInfo{
		"my_rule": {
			MatchAny: false,
			MergeableAttrs: map[string]bool{
				"first_arg":  true,
				"seconf_arg": true,
			},
		},
	}

	for _, tt := range []struct {
		TestName       string
		NewConfig      func() *config.Config
		ExpectedResult *MappedResult
		ExpectedError  error
	}{
		{
			TestName:  "no_mapping",
			NewConfig: config.New,
			ExpectedResult: &MappedResult{
				Kinds: knownKinds,
			},
		},
		{
			TestName: "same_name",
			NewConfig: func() *config.Config {
				c := config.New()
				c.KindMap = map[string]config.MappedKind{
					"my_rule": {
						FromKind: "my_rule",
						KindName: "my_rule",
						KindLoad: "//other:defs.bzl",
					},
				}
				return c
			},
			ExpectedResult: &MappedResult{
				MappedKinds: []config.MappedKind{
					{
						FromKind: "my_rule",
						KindName: "my_rule",
						KindLoad: "//other:defs.bzl",
					},
				},
				Kinds: knownKinds,
			},
		},
		{
			TestName: "different_name",
			NewConfig: func() *config.Config {
				c := config.New()
				c.KindMap = map[string]config.MappedKind{
					"my_rule": {
						FromKind: "my_rule",
						KindName: "other_rule",
						KindLoad: "//other:defs.bzl",
					},
				}
				return c
			},
			ExpectedResult: &MappedResult{
				MappedKinds: []config.MappedKind{
					{
						FromKind: "my_rule",
						KindName: "other_rule",
						KindLoad: "//other:defs.bzl",
					},
				},
				Kinds: map[string]rule.KindInfo{
					"my_rule":    knownKinds["my_rule"],
					"other_rule": knownKinds["my_rule"],
				},
			},
		},
		{
			TestName: "no_first_arg",
			NewConfig: func() *config.Config {
				c := config.New()
				c.KindMap = map[string]config.MappedKind{
					"something": {
						FromKind: "something",
						KindName: "my_something",
						KindLoad: "//other:defs.bzl",
					},
				}
				return c
			},
			ExpectedResult: &MappedResult{
				Kinds: knownKinds,
			},
		},
	} {
		t.Run(tt.TestName, func(t *testing.T) {
			c := tt.NewConfig()

			result, err := ApplyOnRules(c, knownKinds, f)

			if !cmp.Equal(tt.ExpectedError, err) {
				if err != nil {
					t.Errorf("An unexpected error is returned: %v", err)
				} else {
					t.Errorf("An error was expected but no error has been returned, expected error: %v", tt.ExpectedError)
				}
			}

			if diff := cmp.Diff(tt.ExpectedResult, result); diff != "" {
				t.Errorf("(-want, +got):\n%s", diff)
			}
		})
	}
}
