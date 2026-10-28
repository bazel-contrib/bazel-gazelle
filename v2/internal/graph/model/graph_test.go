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

package model

import (
	"bytes"
	"errors"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/google/go-cmp/cmp"
)

func tk(s string) NodeKey {
	l, err := label.Parse(s)
	if err != nil {
		panic(err)
	}
	return TargetKey(l)
}

func newBuilder(t *testing.T, schemas ...EdgeSchema) *Builder {
	t.Helper()
	b, err := NewBuilder(BuilderOptions{Schemas: schemas})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustAdd(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func freeze(t *testing.T, b *Builder) *Graph {
	t.Helper()
	g, err := b.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func keys(ns []Node) []NodeKey {
	var ks []NodeKey
	for _, n := range ns {
		ks = append(ks, n.Key)
	}
	return ks
}

func depKeys(ds []Dep) []NodeKey {
	var ks []NodeKey
	for _, d := range ds {
		ks = append(ks, d.Node.Key)
	}
	return ks
}

func TestKeys(t *testing.T) {
	for _, tc := range []struct {
		label string
		want  NodeKey
	}{
		{"//a/b", "t:@@//a/b:b"},
		{"//a/b:c", "t:@@//a/b:c"},
		{"@//a:c", "t:@@//a:c"},
		{"@@//a:c", "t:@@//a:c"},
		{"@foo//a:c", "t:@foo//a:c"},
		{"@@foo+//a:c", "t:@@foo+//a:c"},
		{"//:root", "t:@@//:root"},
		{"//a:ünïcode", "t:@@//a:ünïcode"},
	} {
		got := tk(tc.label)
		if got != tc.want {
			t.Errorf("TargetKey(%s) = %q, want %q", tc.label, got, tc.want)
		}
		if _, err := ParseKey(got); err != nil {
			t.Errorf("ParseKey(%q): %v", got, err)
		}
	}
	if k := ImportKey(ImportSpec{Lang: "go", Imp: "example.com/a:b"}); k != "i:go:example.com/a:b" {
		t.Errorf("ImportKey = %q", k)
	}
	info, err := ParseKey("i:go:example.com/a:b")
	if err != nil || info.Import.Imp != "example.com/a:b" || info.Class != KindImport {
		t.Errorf("ParseKey(import) = %+v, %v", info, err)
	}
	for _, bad := range []NodeKey{"", "t://a", "t:@@//a/b", "t:@@//a:b:c", "x:@@//a:b", "i:go:", "i::x", "t::rel"} {
		if _, err := ParseKey(bad); !errors.Is(err, ErrNonCanonicalKey) {
			t.Errorf("ParseKey(%q) = %v, want ErrNonCanonicalKey", bad, err)
		}
	}
	rel := TargetKey(label.Label{Name: "x", Relative: true})
	if _, err := ParseKey(rel); err == nil {
		t.Errorf("relative label key %q accepted", rel)
	}
}

func TestNodeKinds(t *testing.T) {
	b := newBuilder(t)
	mustAdd(t, b.AddNode(Node{Key: tk("//a"), RuleKind: "go_library", Lang: "go", Origin: OriginExisting}))
	mustAdd(t, b.AddNode(Node{Key: tk("@ext//x")}))
	if err := b.AddNode(Node{Key: tk("//a"), Kind: KindSourceFile}); !errors.Is(err, ErrConflictingNode) {
		t.Errorf("conflicting kind: got %v", err)
	}
	// Generated facts win RuleKind conflicts; origins are merged.
	mustAdd(t, b.AddNode(Node{Key: tk("//a"), RuleKind: "go_test", Origin: OriginGenerated}))
	mustAdd(t, b.AddNode(Node{Key: tk("//a"), RuleKind: "other", Origin: OriginExisting}))
	g := freeze(t, b)
	n, _ := g.Node(tk("//a"))
	if n.Kind != KindTarget || n.RuleKind != "go_test" || n.Lang != "go" || n.Origin != OriginExisting|OriginGenerated {
		t.Errorf("merged node = %+v", n)
	}
	if n, _ := g.Node(tk("@ext//x")); n.Kind != KindExternal {
		t.Errorf("external kind = %v", n.Kind)
	}
}

func TestEdgeValidation(t *testing.T) {
	b := newBuilder(t, EdgeSchema{Kind: "py/conftest"})
	imp := ImportKey(ImportSpec{Lang: "go", Imp: "x"})
	src := SourceFileKey(label.New("", "a", "a.go"))
	for _, tc := range []struct {
		name     string
		from, to NodeKey
		e        Edge
		want     error
	}{
		{"provides to target", tk("//a"), tk("//b"), Provides{}, ErrInvalidEdge},
		{"depends on import", tk("//a"), imp, DependsOn{Attr: "deps"}, ErrInvalidEdge},
		{"from external", tk("@x//a"), tk("//b"), DependsOn{Attr: "deps"}, ErrInvalidEdge},
		{"generates source", tk("//a"), src, Generates{}, ErrInvalidEdge},
		{"unregistered ext", tk("//a"), tk("//b"), ExtensionEdge{Kind: "py/other"}, ErrUnknownEdgeKind},
		{"malformed ext", tk("//a"), tk("//b"), ExtensionEdge{Kind: "bare"}, ErrUnknownEdgeKind},
		{"bad key", "t:nope", tk("//b"), DependsOn{}, ErrNonCanonicalKey},
		{"nil", tk("//a"), tk("//b"), nil, ErrInvalidEdge},
		{"ok contains", tk("//a"), src, Contains{Attr: "srcs"}, nil},
		{"ok ext", tk("//a"), tk("//b"), ExtensionEdge{Kind: "py/conftest"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := b.AddEdge(tc.from, tc.to, tc.e, OriginGenerated)
			if !errors.Is(err, tc.want) && !(tc.want == nil && err == nil) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := NewBuilder(BuilderOptions{Schemas: []EdgeSchema{{Kind: "nolang"}}}); !errors.Is(err, ErrUnknownEdgeKind) {
		t.Errorf("bad schema: %v", err)
	}
}

func TestEdgeMergeIsIdempotent(t *testing.T) {
	b := newBuilder(t)
	a, c := tk("//a"), tk("//c")
	mustAdd(t, b.AddEdge(a, c, DependsOn{Attr: "deps", Reason: []ImportSpec{{Lang: "go", Imp: "z"}, {Lang: "go", Imp: "y"}}}, OriginGenerated))
	mustAdd(t, b.AddEdge(a, c, DependsOn{Attr: "deps", Reason: []ImportSpec{{Lang: "go", Imp: "y"}}}, OriginExisting))
	mustAdd(t, b.AddEdge(a, c, DependsOn{Attr: "data"}, OriginExisting))
	g := freeze(t, b)
	ds := g.DirectDeps(a, DepQuery{})
	want := []Dep{
		{Node: g.node(c), Attr: "data", Origin: OriginExisting},
		{Node: g.node(c), Attr: "deps", Reason: []ImportSpec{{Lang: "go", Imp: "y"}, {Lang: "go", Imp: "z"}}, Origin: OriginExisting | OriginGenerated},
	}
	if diff := cmp.Diff(want, ds); diff != "" {
		t.Errorf("DirectDeps (-want +got):\n%s", diff)
	}
}

// TestDeterminism checks that insertion order doesn't affect the graph.
func TestDeterminism(t *testing.T) {
	type fact struct {
		from, to NodeKey
		e        Edge
	}
	var facts []fact
	for i := range 20 {
		from := tk("//p" + string(rune('a'+i%7)))
		to := tk("//p" + string(rune('a'+(i*3)%7)))
		facts = append(facts, fact{from, to, DependsOn{Attr: "deps", Reason: []ImportSpec{{Lang: "go", Imp: string(rune('a' + i))}}}})
		facts = append(facts, fact{from, ImportKey(ImportSpec{Lang: "go", Imp: string(from)}), Provides{}})
	}
	var first [32]byte
	var firstJSON string
	for i := range 10 {
		r := rand.New(rand.NewSource(int64(i)))
		r.Shuffle(len(facts), func(a, b int) { facts[a], facts[b] = facts[b], facts[a] })
		b := newBuilder(t)
		for _, f := range facts {
			mustAdd(t, b.AddEdge(f.from, f.to, f.e, OriginGenerated))
		}
		g := freeze(t, b)
		var buf bytes.Buffer
		if err := g.WriteJSON(&buf); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first, firstJSON = g.fingerprint(), buf.String()
			continue
		}
		if g.fingerprint() != first || buf.String() != firstJSON {
			t.Fatalf("shuffle %d produced a different graph", i)
		}
	}
}

func TestFingerprintIgnoresOrigin(t *testing.T) {
	build := func(o Origin) [32]byte {
		b := newBuilder(t)
		mustAdd(t, b.AddNode(Node{Key: tk("//a"), RuleKind: "go_library", Origin: o}))
		mustAdd(t, b.AddEdge(tk("//a"), tk("//b"), DependsOn{Attr: "deps"}, o))
		return freeze(t, b).fingerprint()
	}
	if build(OriginGenerated) != build(OriginExisting) {
		t.Error("fingerprint depends on origin")
	}
}

func TestImmutability(t *testing.T) {
	b := newBuilder(t)
	reason := []ImportSpec{{Lang: "go", Imp: "x"}}
	mustAdd(t, b.AddEdge(tk("//a"), tk("//b"), DependsOn{Attr: "deps", Reason: reason}, OriginGenerated))
	g := freeze(t, b)
	reason[0].Imp = "mutated"
	ds := g.DirectDeps(tk("//a"), DepQuery{})
	ds[0].Reason[0].Imp = "mutated again"
	if got := g.DirectDeps(tk("//a"), DepQuery{})[0].Reason[0].Imp; got != "x" {
		t.Errorf("graph was mutated through a slice: %q", got)
	}
	if err := b.AddNode(Node{Key: tk("//c")}); !errors.Is(err, ErrFrozen) {
		t.Errorf("AddNode after Freeze: %v", err)
	}
	if _, err := b.Freeze(); !errors.Is(err, ErrFrozen) {
		t.Errorf("second Freeze: %v", err)
	}
}

func TestQueries(t *testing.T) {
	b := newBuilder(t, EdgeSchema{Kind: "x/rt", Transitive: true})
	a, lib, libEmb, ext, tool := tk("//a"), tk("//lib"), tk("//lib:emb"), tk("@ext//e"), tk("//tool")
	gen := GeneratedFileKey(label.New("", "tool", "out.go"))
	src := SourceFileKey(label.New("", "a", "a.go"))
	imp := ImportSpec{Lang: "go", Imp: "example.com/lib"}
	for _, n := range []Node{
		{Key: a, Lang: "go"}, {Key: lib, Lang: "go"}, {Key: libEmb, Lang: "go"}, {Key: tool, Lang: "go"},
	} {
		mustAdd(t, b.AddNode(n))
	}
	mustAdd(t, b.AddEdge(lib, ImportKey(imp), Provides{}, OriginGenerated))
	mustAdd(t, b.AddEdge(libEmb, ImportKey(imp), Provides{}, OriginGenerated))
	mustAdd(t, b.AddEdge(lib, libEmb, Embeds{}, OriginGenerated))
	mustAdd(t, b.AddEdge(a, lib, DependsOn{Attr: "deps"}, OriginGenerated))
	mustAdd(t, b.AddEdge(a, ext, DependsOn{Attr: "deps", Select: "@rules_go//go/platform:linux"}, OriginGenerated))
	mustAdd(t, b.AddEdge(a, src, Contains{Attr: "srcs"}, OriginGenerated))
	mustAdd(t, b.AddEdge(a, gen, DependsOn{Attr: "data"}, OriginGenerated))
	mustAdd(t, b.AddEdge(tool, gen, Generates{}, OriginGenerated))
	mustAdd(t, b.AddEdge(lib, tool, ExtensionEdge{Kind: "x/rt", Key: "k", Data: "d"}, OriginGenerated))
	g := freeze(t, b)

	check := func(name string, got, want any) {
		t.Helper()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s (-want +got):\n%s", name, diff)
		}
	}
	check("Providers", keys(g.Providers(imp)), []NodeKey{lib})
	check("ProvidedBy", g.ProvidedBy(libEmb), []ImportSpec{imp})
	check("DirectDeps all", depKeys(g.DirectDeps(a, DepQuery{})), []NodeKey{gen, lib, ext})
	check("DirectDeps unconditional deps", depKeys(g.DirectDeps(a, DepQuery{Attrs: []string{"deps"}, Select: Unconditional})), []NodeKey{lib})
	check("DirectDeps conditional", depKeys(g.DirectDeps(a, DepQuery{Select: SelectKey("@rules_go//go/platform:linux")})), []NodeKey{ext})
	check("DirectDeps kinds", depKeys(g.DirectDeps(a, DepQuery{Kinds: []NodeKind{KindExternal}})), []NodeKey{ext})
	check("DirectRdeps", depKeys(g.DirectRdeps(lib, DepQuery{})), []NodeKey{a})
	check("TransitiveDeps", keys(g.TransitiveDeps(a, DepQuery{Attrs: []string{"deps"}})), []NodeKey{lib, tool, ext})
	path, ok := g.Reachable(a, tool, DepQuery{})
	check("Reachable", path, []NodeKey{a, lib, tool})
	if !ok {
		t.Error("Reachable: not ok")
	}
	if _, ok := g.Reachable(tool, a, DepQuery{}); ok {
		t.Error("Reachable(tool, a) = true")
	}
	check("Embedders", keys(g.Embedders(libEmb)), []NodeKey{lib})
	check("EmbeddedBy", keys(g.EmbeddedBy(lib)), []NodeKey{libEmb})
	if !g.IsSelfImport(libEmb, lib) || g.IsSelfImport(lib, libEmb) || g.IsSelfImport(a, lib) {
		t.Error("IsSelfImport wrong")
	}
	check("Sources", keys(g.Sources(a, "srcs")), []NodeKey{src})
	check("Outputs", keys(g.Outputs(tool)), []NodeKey{gen})
	gr, _ := g.GeneratingRule(gen)
	check("GeneratingRule", gr.Key, tool)
	check("ExtensionNeighbors", g.ExtensionNeighbors(tool, "x/rt", In), []ExtensionDep{{Node: g.node(lib), Key: "k", Data: "d"}})
	check("TargetsInPackage", keys(g.TargetsInPackage("lib")), []NodeKey{libEmb, lib})
	check("RelativeLabels", RelativeLabels(g.DirectDeps(a, DepQuery{Attrs: []string{"deps"}}), label.New("", "a", "a")),
		[]label.Label{{Pkg: "lib", Name: "lib"}, {Repo: "ext", Pkg: "e", Name: "e"}})
	order, err := g.TopoOrder(DepQuery{})
	if err != nil {
		t.Fatal(err)
	}
	pos := map[NodeKey]int{}
	for i, k := range order {
		pos[k] = i
	}
	if pos[lib] > pos[a] || pos[ext] > pos[a] {
		t.Errorf("TopoOrder put a dependency after its dependent: %v", order)
	}
}

func TestLinkAndUnresolved(t *testing.T) {
	b := newBuilder(t)
	a := tk("//a")
	facts := b.PackageFacts("go", "a", OriginGenerated)
	mustAdd(t, facts.Requires(a, ImportSpec{Lang: "go", Imp: "x"}, Requires{Attr: "deps"}))
	mustAdd(t, facts.Requires(a, ImportSpec{Lang: "go", Imp: "y"}, Requires{Attr: "deps"}))
	mustAdd(t, facts.Requires(a, ImportSpec{Lang: "go", Imp: "self"}, Requires{Attr: "deps"}))
	if _, err := b.Freeze(); !errors.Is(err, ErrUnlinked) {
		t.Fatalf("Freeze with pending requires: %v", err)
	}

	b = newBuilder(t)
	facts = b.PackageFacts("go", "a", OriginGenerated)
	mustAdd(t, facts.Requires(a, ImportSpec{Lang: "go", Imp: "x"}, Requires{Attr: "deps"}))
	mustAdd(t, facts.Requires(a, ImportSpec{Lang: "go", Imp: "y"}, Requires{Attr: "deps"}))
	mustAdd(t, facts.Requires(a, ImportSpec{Lang: "go", Imp: "self"}, Requires{Attr: "deps"}))
	pending := b.PendingRequires()
	if len(pending) != 3 || pending[0].Import.Imp != "self" {
		t.Fatalf("PendingRequires = %+v", pending)
	}
	mustAdd(t, b.Link(pending[0], []NodeKey{a}, OriginGenerated))
	mustAdd(t, b.Link(pending[1], []NodeKey{tk("//x")}, OriginGenerated))
	mustAdd(t, b.MarkUnresolved(pending[2], ErrUnresolvedImport))
	g := freeze(t, b)
	if got := depKeys(g.DirectDeps(a, DepQuery{})); !cmp.Equal(got, []NodeKey{tk("//x")}) {
		t.Errorf("deps after link = %v", got)
	}
	if us := g.Unresolved(a); len(us) != 1 || us[0].Import.Imp != "y" || !errors.Is(us[0].Err, ErrUnresolvedImport) {
		t.Errorf("Unresolved = %+v", us)
	}
}

func TestPackageFactsScope(t *testing.T) {
	b := newBuilder(t, EdgeSchema{Kind: "py/x"}, EdgeSchema{Kind: "go/x"})
	f := b.PackageFacts("go", "a", OriginGenerated)
	if err := f.Provides(tk("//b"), ImportSpec{Lang: "go", Imp: "b"}); !errors.Is(err, ErrOutOfPackage) {
		t.Errorf("out of package: %v", err)
	}
	if err := f.Provides(tk("@x//a"), ImportSpec{Lang: "go", Imp: "b"}); !errors.Is(err, ErrOutOfPackage) {
		t.Errorf("external: %v", err)
	}
	if err := f.Edge(tk("//a"), tk("//b"), ExtensionEdge{Kind: "py/x"}); !errors.Is(err, ErrUnknownEdgeKind) {
		t.Errorf("other lang edge: %v", err)
	}
	mustAdd(t, f.Edge(tk("//a"), tk("//b"), ExtensionEdge{Kind: "go/x"}))
}

func TestCycles(t *testing.T) {
	b := newBuilder(t, EdgeSchema{Kind: "x/dep", Dependency: true}, EdgeSchema{Kind: "x/info"})
	dep := func(from, to string, sel string) {
		mustAdd(t, b.AddEdge(tk(from), tk(to), DependsOn{Attr: "deps", Select: sel, Reason: []ImportSpec{{Lang: "go", Imp: to}}}, OriginGenerated))
	}
	// Unconditional 3-cycle.
	dep("//a", "//b", "")
	dep("//b", "//c", "")
	dep("//c", "//a", "")
	dep("//a", "//c", "") // shortcut; the shortest cycle is a -> c -> a.
	// Cycle only under one select key.
	dep("//l", "//m", "linux")
	dep("//m", "//l", "linux")
	// Edges in exclusive branches: not a cycle.
	dep("//p", "//q", "linux")
	dep("//q", "//p", "windows")
	// Self loop.
	dep("//s", "//s", "")
	// Cycle through a generated file.
	gen := GeneratedFileKey(label.New("", "g", "out.txt"))
	mustAdd(t, b.AddEdge(tk("//g"), gen, Generates{}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//g"), tk("//h"), DependsOn{Attr: "tools"}, OriginExisting|OriginKeep))
	mustAdd(t, b.AddEdge(tk("//h"), gen, DependsOn{Attr: "data"}, OriginGenerated))
	// Extension edges: only Dependency kinds count.
	mustAdd(t, b.AddEdge(tk("//x1"), tk("//x2"), ExtensionEdge{Kind: "x/dep"}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//x2"), tk("//x1"), ExtensionEdge{Kind: "x/dep"}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//y1"), tk("//y2"), ExtensionEdge{Kind: "x/info"}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//y2"), tk("//y1"), ExtensionEdge{Kind: "x/info"}, OriginGenerated))
	g := freeze(t, b)

	cycles := g.Cycles()
	type summary struct {
		Nodes  []NodeKey
		Path   []NodeKey
		Select string
	}
	var got []summary
	for _, c := range cycles {
		s := summary{Nodes: c.Nodes, Select: c.Select}
		for _, e := range c.Path {
			s.Path = append(s.Path, e.From)
		}
		s.Path = append(s.Path, c.Path[len(c.Path)-1].To)
		got = append(got, s)
	}
	want := []summary{
		{Nodes: []NodeKey{tk("//a"), tk("//b"), tk("//c")}, Path: []NodeKey{tk("//a"), tk("//c"), tk("//a")}},
		{Nodes: []NodeKey{tk("//g"), tk("//h")}, Path: []NodeKey{tk("//g"), tk("//h"), tk("//g")}},
		{Nodes: []NodeKey{tk("//l"), tk("//m")}, Path: []NodeKey{tk("//l"), tk("//m"), tk("//l")}, Select: "linux"},
		{Nodes: []NodeKey{tk("//s")}, Path: []NodeKey{tk("//s"), tk("//s")}},
		{Nodes: []NodeKey{tk("//x1"), tk("//x2")}, Path: []NodeKey{tk("//x1"), tk("//x2"), tk("//x1")}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Cycles (-want +got):\n%s", diff)
	}

	msg := (&CycleError{Cycle: cycles[1]}).Error()
	for _, s := range []string{"//g --tools [existing|keep]--> //h", "//h --data [via //g:out.txt, generated]--> //g"} {
		if !strings.Contains(msg, s) {
			t.Errorf("cycle message missing %q:\n%s", s, msg)
		}
	}
	if !errors.Is(&CycleError{}, ErrCycle) {
		t.Error("CycleError doesn't wrap ErrCycle")
	}
	if _, err := g.TopoOrder(DepQuery{}); !errors.Is(err, ErrCycle) {
		t.Errorf("TopoOrder on cyclic graph: %v", err)
	}
}

func TestDuplicateOutput(t *testing.T) {
	b := newBuilder(t)
	gen := GeneratedFileKey(label.New("", "g", "out"))
	mustAdd(t, b.AddEdge(tk("//g:a"), gen, Generates{}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//g:b"), gen, Generates{}, OriginGenerated))
	if _, err := b.Freeze(); !errors.Is(err, ErrDuplicateOutput) {
		t.Errorf("Freeze: %v", err)
	}
}

func TestWriteDOT(t *testing.T) {
	b := newBuilder(t)
	mustAdd(t, b.AddEdge(tk("//a"), tk("//b"), DependsOn{Attr: "deps", Select: "linux"}, OriginGenerated))
	var buf bytes.Buffer
	if err := freeze(t, b).WriteDOT(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"t:@@//a:a" -> "t:@@//b:b" [label="deps if linux"]`) {
		t.Errorf("DOT output:\n%s", buf.String())
	}
}

// Regression tests for review findings.

func TestExtNeighborsRejectsCoreKinds(t *testing.T) {
	b := newBuilder(t)
	mustAdd(t, b.AddEdge(tk("//a"), tk("//b"), DependsOn{Attr: "deps"}, OriginGenerated))
	if got := freeze(t, b).ExtensionNeighbors(tk("//a"), KindDependsOn, Out); got != nil {
		t.Errorf("ExtensionNeighbors(core kind) = %v", got)
	}
}

func TestCyclesNotMutable(t *testing.T) {
	b := newBuilder(t)
	mustAdd(t, b.AddEdge(tk("//a"), tk("//b"), DependsOn{Attr: "deps", Reason: []ImportSpec{{Lang: "go", Imp: "b"}}}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//b"), tk("//a"), DependsOn{Attr: "deps", Reason: []ImportSpec{{Lang: "go", Imp: "a"}}}, OriginGenerated))
	g := freeze(t, b)
	cs := g.Cycles()
	cs[0].Path[0].Reason[0].Imp = "mutated"
	cs[0].Nodes[0] = "mutated"
	if c := g.Cycles()[0]; c.Path[0].Reason[0].Imp == "mutated" || c.Nodes[0] == "mutated" {
		t.Error("Cycles result aliases graph state")
	}
}

func TestCycleThroughGeneratedSrc(t *testing.T) {
	b := newBuilder(t)
	gen := GeneratedFileKey(label.New("", "b", "gen.go"))
	mustAdd(t, b.AddEdge(tk("//b"), gen, Generates{}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//a"), gen, Contains{Attr: "srcs"}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//b"), tk("//a"), DependsOn{Attr: "deps"}, OriginGenerated))
	g := freeze(t, b)
	cs := g.Cycles()
	if len(cs) != 1 || cs[0].Path[0].Via != gen || cs[0].Path[0].Attr != "srcs" {
		t.Fatalf("Cycles = %+v", cs)
	}
	var ce *CycleError
	if _, err := g.TopoOrder(DepQuery{}); !errors.As(err, &ce) || len(ce.Cycle.Path) != 2 {
		t.Errorf("TopoOrder error = %v", err)
	}
}

func TestNodeMergeOrderIndependent(t *testing.T) {
	facts := []Node{
		{Key: tk("//a"), RuleKind: "x", Lang: "l1", Origin: OriginExisting},
		{Key: tk("//a"), RuleKind: "y", Lang: "l2", Origin: OriginIndexed},
		{Key: tk("//a"), RuleKind: "w", Origin: OriginExisting},
	}
	var want Node
	for i := range 6 {
		r := rand.New(rand.NewSource(int64(i)))
		r.Shuffle(len(facts), func(a, b int) { facts[a], facts[b] = facts[b], facts[a] })
		b := newBuilder(t)
		for _, f := range facts {
			mustAdd(t, b.AddNode(f))
		}
		if len(b.Warnings()) == 0 {
			t.Error("no warnings for conflicting facts")
		}
		n, _ := freeze(t, b).Node(tk("//a"))
		if i == 0 {
			want = n
			if n.RuleKind != "w" || n.Lang != "l1" {
				t.Errorf("merged node = %+v, want RuleKind w (existing, smallest), Lang l1", n)
			}
		} else if n != want {
			t.Errorf("order %d: got %+v, want %+v", i, n, want)
		}
	}
}

func TestImportLangWithColonRejected(t *testing.T) {
	b := newBuilder(t)
	f := b.PackageFacts("go", "a", OriginGenerated)
	if err := f.Provides(tk("//a"), ImportSpec{Lang: "a:b", Imp: "c"}); !errors.Is(err, ErrNonCanonicalKey) {
		t.Errorf("Provides with ':' in lang: %v", err)
	}
	if err := f.Requires(tk("//a"), ImportSpec{Lang: "go", Imp: ""}, Requires{}); !errors.Is(err, ErrNonCanonicalKey) {
		t.Errorf("Requires with empty import: %v", err)
	}
}

func TestRejectedEdgeLeavesNoNodes(t *testing.T) {
	b := newBuilder(t)
	if err := b.AddEdge(tk("//a"), ImportKey(ImportSpec{Lang: "go", Imp: "x"}), DependsOn{}, OriginGenerated); err == nil {
		t.Fatal("expected error")
	}
	if g := freeze(t, b); g.NodeCount() != 0 {
		t.Errorf("rejected edge left %d nodes", g.NodeCount())
	}
}

func TestSelectCyclePathUsesSelectEdge(t *testing.T) {
	b := newBuilder(t)
	dep := func(from, to, sel string) {
		mustAdd(t, b.AddEdge(tk(from), tk(to), DependsOn{Attr: "deps", Select: sel}, OriginGenerated))
	}
	dep("//a", "//b", "")
	dep("//b", "//a", "")
	dep("//b", "//c", "linux")
	dep("//c", "//a", "linux")
	cs := freeze(t, b).Cycles()
	if len(cs) != 2 || cs[1].Select != "linux" {
		t.Fatalf("Cycles = %+v", cs)
	}
	if !slices.ContainsFunc(cs[1].Path, func(e CycleEdge) bool { return e.Select == "linux" }) {
		t.Errorf("select cycle path has no select edge: %+v", cs[1].Path)
	}
}

func TestTopoOrderSelect(t *testing.T) {
	b := newBuilder(t)
	mustAdd(t, b.AddEdge(tk("//a"), tk("//b"), DependsOn{Attr: "deps", Select: "linux"}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//b"), tk("//a"), DependsOn{Attr: "deps", Select: "windows"}, OriginGenerated))
	g := freeze(t, b)
	if len(g.Cycles()) != 0 {
		t.Error("exclusive select branches reported as a cycle")
	}
	order, err := g.TopoOrder(DepQuery{Select: SelectKey("linux")})
	if err != nil || !slices.Equal(order, []NodeKey{tk("//b"), tk("//a")}) {
		t.Errorf("TopoOrder(linux) = %v, %v", order, err)
	}
}

func TestConcurrentReads(t *testing.T) {
	b := newBuilder(t)
	mustAdd(t, b.AddEdge(tk("//a"), tk("//b"), DependsOn{Attr: "deps"}, OriginGenerated))
	mustAdd(t, b.AddEdge(tk("//b"), tk("//a"), DependsOn{Attr: "deps"}, OriginGenerated))
	g := freeze(t, b)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.Cycles()
			_ = g.DirectDeps(tk("//a"), DepQuery{})
			_ = g.fingerprint()
		}()
	}
	wg.Wait()
}

func TestProvidersIncludeEmbedders(t *testing.T) {
	b := newBuilder(t)
	x, y, z := tk("//x"), tk("//y"), tk("//z")
	mustAdd(t, b.AddNode(Node{Key: x, Lang: "go"}))
	mustAdd(t, b.AddNode(Node{Key: y, Lang: "proto"}))
	mustAdd(t, b.AddNode(Node{Key: z, Lang: "go"}))
	mustAdd(t, b.AddEdge(x, ImportKey(ImportSpec{Lang: "go", Imp: "x"}), Provides{}, OriginGenerated))
	mustAdd(t, b.AddEdge(y, ImportKey(ImportSpec{Lang: "go", Imp: "y"}), Provides{}, OriginGenerated))
	mustAdd(t, b.AddEdge(x, y, Embeds{}, OriginGenerated))
	mustAdd(t, b.AddEdge(z, x, Embeds{}, OriginGenerated))
	g := freeze(t, b)
	// z embeds x embeds y: all provide "y"; y is a different language than
	// its embedders so it stays; x is embedded by z (same language).
	if got := keys(g.Providers(ImportSpec{Lang: "go", Imp: "y"})); !slices.Equal(got, []NodeKey{y, z}) {
		t.Errorf("Providers(y) = %v", got)
	}
	if got := keys(g.Providers(ImportSpec{Lang: "go", Imp: "x"})); !slices.Equal(got, []NodeKey{z}) {
		t.Errorf("Providers(x) = %v", got)
	}
}
