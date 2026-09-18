// Language test_same_kind_2 is used in //tests:same_kind to verify that
// two extensions can generate rules with the same kind name.
// See tests/same_kind/README.md.
package test_same_kind_2

import (
	"context"
	"fmt"

	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

func NewV2() language.Language {
	return lang{}
}

type lang struct{}

var _ language.Language = lang{}
var _ language.Generator = lang{}
var _ resolve.Indexer = lang{}
var _ resolve.Resolver = lang{}

const langName = "test_same_kind_2"

func (lang) Name() string {
	return langName
}

var kinds = []rule.KindInfo{
	{
		Name:           "same_kind",
		LoadedFrom:     label.New("", "", "same_kind_2.bzl"),
		MatchAttrs:     []string{"match_attr_2"},
		MergeableAttrs: map[string]bool{"merge_attr_2": true},
		ResolveAttrs:   map[string]bool{"resolve_attr_2": true},
	},
}

func (lang) Kinds() []rule.KindInfo {
	return kinds
}

func (lang) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	r := rule.NewRule("same_kind", "r2")
	r.SetAttr("match_attr_2", "match_r2")
	r.SetAttr("merge_attr_2", "merge_r2")
	return language.GenerateResult{
		Gen: []*rule.Rule{r},
	}, nil
}

func (lang) Imports(ctx context.Context, args resolve.ImportsArgs) (resolve.ImportsResult, error) {
	r := args.Rule
	match := r.AttrString("match_attr_2")
	if match == "" {
		return resolve.ImportsResult{}, fmt.Errorf("Imports called unexpectedly on rule %s without match_attr_2", r.Name())
	}
	return resolve.ImportsResult{
		Imports: []resolve.ImportSpec{{
			Lang: langName,
			Imp:  match,
		}},
	}, nil
}

func (lang) Resolve(ctx context.Context, args resolve.ResolveArgs) error {
	r := args.Rule
	if r.AttrString("match_attr_2") == "" {
		return fmt.Errorf("Resolve called unexpectedly on rule %s without match_attr_2", r.Name())
	}
	r.SetAttr("resolve_attr_2", "resolve_r2")
	return nil
}
