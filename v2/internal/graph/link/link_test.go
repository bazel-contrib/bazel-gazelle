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

package link

import (
	"context"
	"errors"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/graph"
	"github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/ingest"
	model "github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/model"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/google/go-cmp/cmp"
)

type fakeFinder struct{ results map[string][]label.Label }

func (fakeFinder) Name() string { return "fake" }

func (f fakeFinder) Find(_ context.Context, args resolve.FindArgs) ([]resolve.FindResult, error) {
	var rs []resolve.FindResult
	for _, l := range f.results[args.Import.Imp] {
		rs = append(rs, resolve.FindResult{Label: l})
	}
	return rs, nil
}

type stdlibPolicy struct{}

func (stdlibPolicy) Link(ctx context.Context, args graph.LinkArgs) (graph.LinkResult, error) {
	if args.Import.Imp == "fmt" {
		return graph.LinkResult{}, nil // standard library: no dependency
	}
	if len(args.Candidates) == 0 {
		rs, err := args.Find(ctx)
		if err != nil || len(rs) == 0 {
			return graph.LinkResult{Err: graph.ErrUnresolvedImport}, nil
		}
		return graph.LinkResult{To: []graph.NodeKey{ingest.LabelKey(rs[0].Label, "main", args.From.Pkg)}}, nil
	}
	return graph.LinkResult{To: []graph.NodeKey{args.Candidates[0].Key}}, nil
}

func configWithOverride(t *testing.T) *config.Config {
	c := &config.Config{Exts: map[string]any{}}
	if err := (&resolve.Configurer{}).Configure(t.Context(), config.ConfigureArgs{
		Config: c,
		File:   &rule.File{Directives: []rule.Directive{{Key: "resolve", Value: "go example.com/over //over:target"}}},
	}); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLink(t *testing.T) {
	b, _ := model.NewBuilder(model.BuilderOptions{})
	k := func(pkg, name string) graph.NodeKey { return ingest.RuleKey(pkg, name) }
	add := func(pkg, name, lang string) {
		if err := b.AddNode(graph.Node{Key: k(pkg, name), Lang: lang, Origin: model.OriginGenerated}); err != nil {
			t.Fatal(err)
		}
	}
	add("app", "app", "go")
	add("lib", "lib", "go")
	add("lib", "lib_internal", "go")
	add("dup", "one", "go")
	add("dup", "two", "go")
	add("pb", "go_pb", "proto")
	add("app", "app_embedded", "go")
	add("cli", "cli", "py")

	facts := func(pkg, lang string) graph.Facts { return b.PackageFacts(lang, pkg, model.OriginGenerated) }
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	gi := func(s string) graph.ImportSpec { return graph.ImportSpec{Lang: "go", Imp: s} }
	must(facts("lib", "go").Provides(k("lib", "lib"), gi("example.com/lib")))
	// lib embeds lib_internal, which provides the same import: lib wins.
	must(facts("lib", "go").Provides(k("lib", "lib_internal"), gi("example.com/lib")))
	must(facts("lib", "go").Embeds(k("lib", "lib"), k("lib", "lib_internal")))
	must(facts("dup", "go").Provides(k("dup", "one"), gi("example.com/dup")))
	must(facts("dup", "go").Provides(k("dup", "two"), gi("example.com/dup")))
	// Provided only by the proto extension: cross-language.
	must(facts("pb", "proto").Provides(k("pb", "go_pb"), gi("example.com/pb")))
	// app embeds app_embedded, which provides an import app requires: self.
	must(facts("app", "go").Provides(k("app", "app_embedded"), gi("example.com/app")))
	must(facts("app", "go").Embeds(k("app", "app"), k("app", "app_embedded")))

	app := facts("app", "go")
	for _, imp := range []string{"example.com/lib", "example.com/dup", "example.com/pb", "example.com/app", "example.com/over", "example.com/found", "example.com/missing"} {
		must(app.Requires(k("app", "app"), gi(imp), graph.Requires{Attr: "deps"}))
	}
	cli := facts("cli", "py")
	for _, imp := range []string{"fmt", "example.com/found", "example.com/lib"} {
		must(cli.Requires(k("cli", "cli"), gi(imp), graph.Requires{Attr: "deps"}))
	}

	c := configWithOverride(t)
	err := Link(t.Context(), b, Options{
		RepoName: "main",
		Config:   func(string) *config.Config { return c },
		Policy: func(lang string) graph.LinkPolicy {
			if lang == "py" {
				return stdlibPolicy{}
			}
			return nil
		},
		Accepts: func(lang string) []string {
			if lang == "go" {
				return []string{"proto"}
			}
			return nil
		},
		Finders: []resolve.Finder{fakeFinder{results: map[string][]label.Label{
			"example.com/found": {label.New("found_repo", "", "found_repo")},
		}}},
	})
	must(err)
	g, err := b.Freeze()
	must(err)

	var deps []graph.NodeKey
	for _, d := range g.DirectDeps(k("app", "app"), graph.DepQuery{}) {
		deps = append(deps, d.Node.Key)
	}
	want := []graph.NodeKey{"t:@@//lib:lib", "t:@@//over:target", "t:@@//pb:go_pb", "t:@found_repo//:found_repo"}
	if diff := cmp.Diff(want, deps); diff != "" {
		t.Errorf("app deps (-want +got):\n%s", diff)
	}
	us := g.Unresolved(k("app", "app"))
	if len(us) != 2 || us[0].Import.Imp != "example.com/dup" || !errors.Is(us[0].Err, graph.ErrAmbiguousImport) ||
		us[1].Import.Imp != "example.com/missing" || !errors.Is(us[1].Err, graph.ErrUnresolvedImport) {
		t.Errorf("app unresolved = %+v", us)
	}

	// The py policy drops "fmt", uses Find, and does NOT see go providers
	// (py accepts no other extensions), so example.com/lib is unresolved.
	deps = nil
	for _, d := range g.DirectDeps(k("cli", "cli"), graph.DepQuery{}) {
		deps = append(deps, d.Node.Key)
	}
	if diff := cmp.Diff([]graph.NodeKey{"t:@found_repo//:found_repo"}, deps); diff != "" {
		t.Errorf("cli deps (-want +got):\n%s", diff)
	}
	if us := g.Unresolved(k("cli", "cli")); len(us) != 1 || us[0].Import.Imp != "example.com/lib" {
		t.Errorf("cli unresolved = %+v", us)
	}
}

func TestLinkPrefersOwnLanguage(t *testing.T) {
	b, _ := model.NewBuilder(model.BuilderOptions{})
	for _, n := range []graph.Node{
		{Key: ingest.RuleKey("a", "a"), Lang: "go"},
		{Key: ingest.RuleKey("g", "g"), Lang: "go"},
		{Key: ingest.RuleKey("p", "p"), Lang: "proto"},
	} {
		if err := b.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	c := configWithOverride(t)
	imp := graph.ImportSpec{Lang: "go", Imp: "x"}
	b.PackageFacts("go", "g", 0).Provides(ingest.RuleKey("g", "g"), imp)
	b.PackageFacts("proto", "p", 0).Provides(ingest.RuleKey("p", "p"), imp)
	b.PackageFacts("go", "a", 0).Requires(ingest.RuleKey("a", "a"), imp, graph.Requires{Attr: "deps"})
	if err := Link(t.Context(), b, Options{
		Config:  func(string) *config.Config { return c },
		Accepts: func(string) []string { return []string{"proto"} },
	}); err != nil {
		t.Fatal(err)
	}
	g, _ := b.Freeze()
	if ds := g.DirectDeps(ingest.RuleKey("a", "a"), graph.DepQuery{}); len(ds) != 1 || ds[0].Node.Key != ingest.RuleKey("g", "g") {
		t.Errorf("deps = %+v, want only //g (own language preferred)", ds)
	}
}

// Regression tests for parity with resolve.RuleIndex and the Go resolver.

type linkFixture struct {
	t *testing.T
	b *model.Builder
}

func newFixture(t *testing.T) *linkFixture {
	b, _ := model.NewBuilder(model.BuilderOptions{})
	return &linkFixture{t: t, b: b}
}

func (f *linkFixture) node(pkg, lang string) graph.NodeKey {
	k := ingest.RuleKey(pkg, pkg)
	if err := f.b.AddNode(graph.Node{Key: k, Lang: lang}); err != nil {
		f.t.Fatal(err)
	}
	return k
}

func (f *linkFixture) must(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *linkFixture) facts(k graph.NodeKey, lang string) graph.Facts {
	info, _ := graph.ParseKey(k)
	return f.b.PackageFacts(lang, info.Label.Pkg, model.OriginGenerated)
}

func (f *linkFixture) link(opts Options) *graph.Graph {
	f.t.Helper()
	c := configWithOverride(f.t)
	opts.Config = func(string) *config.Config { return c }
	f.must(Link(f.t.Context(), f.b, opts))
	g, err := f.b.Freeze()
	f.must(err)
	return g
}

func depsOf(g *graph.Graph, k graph.NodeKey) []graph.NodeKey {
	var ks []graph.NodeKey
	for _, d := range g.DirectDeps(k, graph.DepQuery{}) {
		ks = append(ks, d.Node.Key)
	}
	return ks
}

// An embedder provides the imports of the rules it embeds.
func TestLinkEmbedderProvides(t *testing.T) {
	f := newFixture(t)
	x, y, z := f.node("x", "go"), f.node("y", "go"), f.node("z", "go")
	f.must(f.facts(x, "go").Provides(x, graph.ImportSpec{Lang: "go", Imp: "x"}))
	f.must(f.facts(y, "go").Provides(y, graph.ImportSpec{Lang: "go", Imp: "y"}))
	f.must(f.facts(x, "go").Embeds(x, y))
	f.must(f.facts(z, "go").Requires(z, graph.ImportSpec{Lang: "go", Imp: "y"}, graph.Requires{Attr: "deps"}))
	g := f.link(Options{})
	if diff := cmp.Diff([]graph.NodeKey{x}, depsOf(g, z)); diff != "" {
		t.Errorf("z deps (-want +got):\n%s", diff)
	}
}

// A rule embedded by X that imports X's import doesn't depend on X.
func TestLinkEmbeddedRequiresEmbedder(t *testing.T) {
	f := newFixture(t)
	x, y := f.node("x", "go"), f.node("y", "go")
	f.must(f.facts(x, "go").Provides(x, graph.ImportSpec{Lang: "go", Imp: "x"}))
	f.must(f.facts(x, "go").Embeds(x, y))
	f.must(f.facts(y, "go").Requires(y, graph.ImportSpec{Lang: "go", Imp: "x"}, graph.Requires{Attr: "deps"}))
	g := f.link(Options{})
	if deps := depsOf(g, y); len(deps) != 0 {
		t.Errorf("y deps = %v, want none (self import)", deps)
	}
	if us := g.Unresolved(y); len(us) != 0 {
		t.Errorf("unresolved = %+v", us)
	}
}

// Ambiguity is reported before the self-import check, like the Go resolver.
func TestLinkAmbiguousWithSelf(t *testing.T) {
	f := newFixture(t)
	a, b := f.node("a", "go"), f.node("b", "go")
	imp := graph.ImportSpec{Lang: "go", Imp: "same"}
	f.must(f.facts(a, "go").Provides(a, imp))
	f.must(f.facts(b, "go").Provides(b, imp))
	f.must(f.facts(a, "go").Requires(a, imp, graph.Requires{Attr: "deps"}))
	g := f.link(Options{})
	if us := g.Unresolved(a); len(us) != 1 || !errors.Is(us[0].Err, graph.ErrAmbiguousImport) {
		t.Errorf("unresolved = %+v, want ambiguous", us)
	}
}

type selfPolicy struct{ sawSelf *bool }

func (p selfPolicy) Link(_ context.Context, args graph.LinkArgs) (graph.LinkResult, error) {
	*p.sawSelf = args.SelfImport
	return graph.LinkResult{To: []graph.NodeKey{args.FromNode}}, nil
}

// Policies learn about removed self imports, and self labels they return
// are dropped.
func TestLinkPolicySelf(t *testing.T) {
	f := newFixture(t)
	a := f.node("a", "go")
	imp := graph.ImportSpec{Lang: "go", Imp: "a"}
	f.must(f.facts(a, "go").Provides(a, imp))
	f.must(f.facts(a, "go").Requires(a, imp, graph.Requires{Attr: "deps"}))
	var sawSelf bool
	g := f.link(Options{Policy: func(string) graph.LinkPolicy { return selfPolicy{&sawSelf} }})
	if !sawSelf {
		t.Error("LinkArgs.SelfImport = false")
	}
	if deps := depsOf(g, a); len(deps) != 0 {
		t.Errorf("deps = %v, want none", deps)
	}
}

type failingPolicy struct{}

func (failingPolicy) Link(context.Context, graph.LinkArgs) (graph.LinkResult, error) {
	return graph.LinkResult{}, context.Canceled
}

// A hard policy error aborts linking instead of being recorded.
func TestLinkPolicyError(t *testing.T) {
	f := newFixture(t)
	a := f.node("a", "go")
	f.must(f.facts(a, "go").Requires(a, graph.ImportSpec{Lang: "go", Imp: "x"}, graph.Requires{Attr: "deps"}))
	c := configWithOverride(t)
	err := Link(t.Context(), f.b, Options{
		Config: func(string) *config.Config { return c },
		Policy: func(string) graph.LinkPolicy { return failingPolicy{} },
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Link error = %v, want context.Canceled", err)
	}
}
