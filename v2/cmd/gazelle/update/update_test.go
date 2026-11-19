package update

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/compat"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/bazelbuild/bazel-gazelle/config"
	languagev1 "github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
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

	langs := []language.Language{compat.LanguageV2(mapKindTestLanguage{indexed: &indexed})}
	if err := Run(context.Background(), langs, dir, nil); err != nil {
		t.Fatal(err)
	}
	if !indexed {
		t.Fatal("configured mapped rule was not indexed by its source-kind resolver")
	}
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

func (mapKindTestLanguage) GenerateRules(args languagev1.GenerateArgs) languagev1.GenerateResult {
	if args.Rel != "" {
		return languagev1.GenerateResult{}
	}
	return languagev1.GenerateResult{
		Gen: []*rule.Rule{rule.NewRule("mapped_library", "generated")},
	}
}

func (mapKindTestLanguage) Resolve(_ *config.Config, _ *resolve.RuleIndex, _ *repo.RemoteCache, r *rule.Rule, _ any, _ label.Label) {
}

// TestAliasedKindGeneratedRuleMergesAttrs verifies that aliases' attributes can
// be updated (in addition to being added).
func TestAliasedKindGeneratedRuleMergesAttrs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "WORKSPACE"), nil, 0o666); err != nil {
		t.Fatal(err)
	}
	buildPath := filepath.Join(dir, "BUILD.bazel")
	if err := os.WriteFile(buildPath, []byte(`# gazelle:alias_kind aliased_library base_library

aliased_library(
    name = "foo",
    srcs = ["old.src"],
    deps = [":old"],
)
`), 0o666); err != nil {
		t.Fatal(err)
	}

	langs := []language.Language{compat.LanguageV2(aliasKindTestLanguage{})}
	if err := Run(context.Background(), langs, dir, nil); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatal(err)
	}
	want := `# gazelle:alias_kind aliased_library base_library

aliased_library(
    name = "foo",
    srcs = ["new.src"],
    deps = [":new"],
)
`
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// aliasKindTestLanguage declares base_library, but generates a rule with
// aliased_library kind, as some languages (for example, gazelle_cc) do.
type aliasKindTestLanguage struct{}

func (aliasKindTestLanguage) Name() string { return "alias_kind_test" }

func (aliasKindTestLanguage) RegisterFlags(*flag.FlagSet, string, *config.Config) {}

func (aliasKindTestLanguage) CheckFlags(*flag.FlagSet, *config.Config) error { return nil }

func (aliasKindTestLanguage) KnownDirectives() []string { return nil }

func (aliasKindTestLanguage) Configure(*config.Config, string, *rule.File) {}

func (aliasKindTestLanguage) Kinds() map[string]rule.KindInfo {
	return map[string]rule.KindInfo{
		"base_library": {
			MatchAny:       true,
			MergeableAttrs: map[string]bool{"srcs": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		},
	}
}

func (aliasKindTestLanguage) Loads() []rule.LoadInfo { return nil }

func (aliasKindTestLanguage) Fix(*config.Config, *rule.File) {}

func (aliasKindTestLanguage) Imports(*config.Config, *rule.Rule, *rule.File) []resolve.ImportSpec {
	return nil
}

func (aliasKindTestLanguage) Embeds(*rule.Rule, label.Label) []label.Label { return nil }

func (aliasKindTestLanguage) GenerateRules(args languagev1.GenerateArgs) languagev1.GenerateResult {
	if args.Rel != "" {
		return languagev1.GenerateResult{}
	}
	// Languages may generate a rule of an aliased kind directly, rather than
	// the underlying kind.
	r := rule.NewRule("aliased_library", "foo")
	r.SetAttr("srcs", []string{"new.src"})
	return languagev1.GenerateResult{
		Gen:     []*rule.Rule{r},
		Imports: []any{nil},
	}
}

func (aliasKindTestLanguage) Resolve(_ *config.Config, _ *resolve.RuleIndex, _ *repo.RemoteCache, r *rule.Rule, _ any, _ label.Label) {
	r.SetAttr("deps", []string{":new"})
}
