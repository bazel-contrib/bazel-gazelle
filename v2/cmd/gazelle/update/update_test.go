package update

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/testtools"
)

// TestConfiguredMappedKindResolvesWithoutSourceRule verifies that a configured
// map_kind is registered even when no rule of its source kind was generated
// in the package.
func TestConfiguredMappedKindResolvesWithoutSourceRule(t *testing.T) {
	indexed := false
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "WORKSPACE"), nil, 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "BUILD.bazel"), []byte(`
# gazelle:map_kind base_library mapped_library //tools:defs.bzl
`), 0o666); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), []language.Language{mapKindTestLanguage{indexed: &indexed}}, dir, nil); err != nil {
		t.Fatal(err)
	}
	if !indexed {
		t.Fatal("configured mapped rule was not indexed by its source-kind resolver")
	}
}

func TestAncestorUpdateDirectories(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{Path: "a/BUILD.bazel"},
		{Path: "a/child/file.txt"},
	})
	defer cleanup()

	var generated, observed []string
	lang := &ancestorUpdateTestLanguage{generated: &generated}
	observer := &ancestorUpdateObserver{generated: &observed}
	args := []string{"-r=false", "a/child"}
	if err := Run(context.Background(), []language.Language{lang, observer}, dir, args); err != nil {
		t.Fatal(err)
	}

	want := []string{"a/child", "a"}
	if !slices.Equal(want, generated) {
		t.Errorf("generated directories = %v, want %v", generated, want)
	}
	if !slices.Equal(want, observed) {
		t.Errorf("observed directories = %v, want %v", observed, want)
	}
}

type ancestorUpdateTestLanguage struct {
	language.BaseLang
	generated *[]string
}

func (*ancestorUpdateTestLanguage) Name() string { return "ancestor_update_test" }

func (l *ancestorUpdateTestLanguage) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	*l.generated = append(*l.generated, args.Rel)
	if args.Rel == "a/child" {
		return language.GenerateResult{RelsToUpdate: []string{"a"}}
	}
	return language.GenerateResult{}
}

type ancestorUpdateObserver struct {
	language.BaseLang
	generated *[]string
}

func (*ancestorUpdateObserver) Name() string { return "ancestor_update_observer" }

func (l *ancestorUpdateObserver) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	*l.generated = append(*l.generated, args.Rel)
	return language.GenerateResult{}
}

// mapKindTestLanguage generates only mapped_library. The base_library mapping
// must therefore be taken from configuration rather than from a generated
// base_library rule.
type mapKindTestLanguage struct {
	indexed *bool
}

func (mapKindTestLanguage) Name() string { return "map_kind_test" }

func (mapKindTestLanguage) RegisterFlags(*flag.FlagSet, string, *config.Config) {}

func (mapKindTestLanguage) CheckFlags(*flag.FlagSet, *config.Config) error { return nil }

func (mapKindTestLanguage) KnownDirectives() []string { return nil }

func (mapKindTestLanguage) Configure(*config.Config, string, *rule.File) {}

func (mapKindTestLanguage) Kinds() map[string]rule.KindInfo {
	return map[string]rule.KindInfo{
		"base_library": {ResolveAttrs: map[string]bool{"deps": true}},
	}
}

func (mapKindTestLanguage) Loads() []rule.LoadInfo { return nil }

func (mapKindTestLanguage) Fix(*config.Config, *rule.File) {}

func (l mapKindTestLanguage) Imports(*config.Config, *rule.Rule, *rule.File) []resolve.ImportSpec {
	*l.indexed = true
	return nil
}

func (mapKindTestLanguage) Embeds(*rule.Rule, label.Label) []label.Label { return nil }

func (mapKindTestLanguage) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	if args.Rel != "" {
		return language.GenerateResult{}
	}
	return language.GenerateResult{
		Gen:     []*rule.Rule{rule.NewRule("mapped_library", "generated")},
		Imports: []any{nil},
	}
}

func (mapKindTestLanguage) Resolve(_ *config.Config, _ *resolve.RuleIndex, _ *repo.RemoteCache, r *rule.Rule, _ any, _ label.Label) {
}
