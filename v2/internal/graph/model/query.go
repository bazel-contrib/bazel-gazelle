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
	"cmp"
	"maps"
	"slices"

	"github.com/bazel-contrib/bazel-gazelle/v2/label"
)

// DepQuery filters DependsOn edges. The zero value matches every edge.
type DepQuery struct {
	// Attrs limits results to these attributes. nil matches all attributes.
	Attrs []string

	// Select limits results by select() key. The zero value matches all
	// keys.
	Select SelectFilter

	// Kinds limits results to dependencies on nodes of these kinds. nil
	// matches all kinds.
	Kinds []NodeKind

	// Origins limits results to edges with at least one of these origin
	// bits. 0 matches all edges.
	Origins Origin
}

// SelectFilter filters dependencies by the select() key they appear under.
// The zero value, AnySelect, matches all dependencies.
type SelectFilter struct {
	key string
	set bool
}

var (
	// AnySelect matches dependencies under any select() key and
	// unconditional dependencies.
	AnySelect = SelectFilter{}

	// Unconditional matches only dependencies outside select().
	Unconditional = SelectFilter{set: true}
)

// SelectKey matches only dependencies under the given select() key.
func SelectKey(key string) SelectFilter { return SelectFilter{key: key, set: true} }

func (f SelectFilter) match(sel string) bool { return !f.set || f.key == sel }

func (q DepQuery) matchEdge(r *edgeRec) bool {
	d := r.e.(DependsOn)
	if q.Attrs != nil && !slices.Contains(q.Attrs, d.Attr) {
		return false
	}
	if !q.Select.match(d.Select) {
		return false
	}
	return q.Origins == 0 || r.origin&q.Origins != 0
}

func (q DepQuery) matchNode(n Node) bool {
	return q.Kinds == nil || slices.Contains(q.Kinds, n.Kind)
}

// Dep is a dependency returned by [Graph.DirectDeps] and [Graph.DirectRdeps].
type Dep struct {
	// Node is the other end of the edge: the dependency for DirectDeps, the
	// dependent for DirectRdeps.
	Node   Node
	Attr   string
	Select string
	Reason []ImportSpec
	Origin Origin
}

func depOf(n Node, r *edgeRec) Dep {
	d := r.e.(DependsOn)
	return Dep{Node: n, Attr: d.Attr, Select: d.Select, Reason: slices.Clone(d.Reason), Origin: r.origin}
}

func sortDeps(ds []Dep) {
	slices.SortFunc(ds, func(a, b Dep) int {
		return cmp.Or(cmp.Compare(a.Attr, b.Attr), cmp.Compare(a.Select, b.Select), cmp.Compare(a.Node.Key, b.Node.Key))
	})
}

// DirectDeps returns the direct dependencies of from matching q, sorted by
// attribute, select key, and node key.
func (g *Graph) DirectDeps(from NodeKey, q DepQuery) []Dep {
	var ds []Dep
	g.outEdges(from, KindDependsOn, func(r *edgeRec) {
		if n := g.node(r.to); q.matchEdge(r) && q.matchNode(n) {
			ds = append(ds, depOf(n, r))
		}
	})
	sortDeps(ds)
	return ds
}

// DirectRdeps returns the nodes that directly depend on to through edges
// matching q, sorted by attribute, select key, and node key. q.Kinds filters
// the dependents.
func (g *Graph) DirectRdeps(to NodeKey, q DepQuery) []Dep {
	var ds []Dep
	g.inEdges(to, KindDependsOn, func(r *edgeRec) {
		if n := g.node(r.from); q.matchEdge(r) && q.matchNode(n) {
			ds = append(ds, depOf(n, r))
		}
	})
	sortDeps(ds)
	return ds
}

// successors returns the nodes reachable from k in one step for transitive
// queries: DependsOn edges matching q, Exports, AliasOf, and extension edges
// whose schema is Transitive. Nodes not matching q.Kinds are skipped.
func (g *Graph) successors(k NodeKey, q DepQuery) []NodeKey {
	i, ok := g.ids[k]
	if !ok {
		return nil
	}
	var next []NodeKey
	for _, ei := range g.out[i] {
		r := &g.edges[ei]
		switch r.kind {
		case KindDependsOn:
			if !q.matchEdge(r) {
				continue
			}
		case KindExports, KindAliasOf:
		default:
			if s, ok := g.schemas[r.kind]; !ok || !s.Transitive {
				continue
			}
		}
		if q.matchNode(g.node(r.to)) {
			next = append(next, r.to)
		}
	}
	return next
}

// TransitiveDeps returns every node reachable from from (excluding from
// itself, unless it's on a cycle) through dependency edges matching q, plus
// Exports, AliasOf, and transitive extension edges. Nodes not matching
// q.Kinds are neither returned nor traversed.
func (g *Graph) TransitiveDeps(from NodeKey, q DepQuery) []Node {
	seen := map[NodeKey]bool{}
	queue := []NodeKey{from}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		for _, n := range g.successors(k, q) {
			if !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	ns := make([]Node, 0, len(seen))
	for k := range seen {
		ns = append(ns, g.node(k))
	}
	slices.SortFunc(ns, func(a, b Node) int { return cmp.Compare(a.Key, b.Key) })
	return ns
}

// Reachable reports whether to is reachable from from by the edges
// TransitiveDeps follows. If so, it returns a shortest path, starting with
// from and ending with to. Among shortest paths, the one visiting
// lexicographically smallest keys first is returned.
func (g *Graph) Reachable(from, to NodeKey, q DepQuery) (path []NodeKey, ok bool) {
	if _, ok := g.ids[from]; !ok {
		return nil, false
	}
	parent := map[NodeKey]NodeKey{}
	seen := map[NodeKey]bool{}
	queue := []NodeKey{from}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		next := g.successors(k, q)
		slices.Sort(next)
		for _, n := range next {
			if seen[n] {
				continue
			}
			seen[n] = true
			parent[n] = k
			if n == to {
				path = []NodeKey{to}
				for c := k; ; c = parent[c] {
					path = append(path, c)
					if c == from {
						break
					}
				}
				slices.Reverse(path)
				return path, true
			}
			queue = append(queue, n)
		}
	}
	return nil, false
}

// Providers returns the targets that provide imp, matching how
// resolve.RuleIndex indexes rules:
//
//   - A target that (transitively) embeds a provider of imp also provides
//     imp, whatever the languages involved.
//   - A target embedded by another provider of imp in the same language is
//     omitted, so the embedding library is preferred.
func (g *Graph) Providers(imp ImportSpec) []Node {
	set := map[NodeKey]Node{}
	g.inEdges(ImportKey(imp), KindProvides, func(r *edgeRec) {
		set[r.from] = g.node(r.from)
	})
	for _, direct := range slices.Collect(maps.Values(set)) {
		for _, e := range g.embeddersOf(direct.Key) {
			set[e.Key] = e
		}
	}
	embedded := map[NodeKey]bool{}
	for _, c := range set {
		for _, e := range g.EmbeddedBy(c.Key) {
			if other, ok := set[e.Key]; ok && other.Lang == c.Lang {
				embedded[e.Key] = true
			}
		}
	}
	var ns []Node
	for k, c := range set {
		if !embedded[k] {
			ns = append(ns, c)
		}
	}
	slices.SortFunc(ns, func(a, b Node) int { return cmp.Compare(a.Key, b.Key) })
	return ns
}

// ProvidedBy returns the imports target provides, sorted.
func (g *Graph) ProvidedBy(target NodeKey) []ImportSpec {
	var imps []ImportSpec
	g.outEdges(target, KindProvides, func(r *edgeRec) {
		imps = append(imps, g.node(r.to).Import)
	})
	slices.SortFunc(imps, compareImports)
	return imps
}

// Unresolved returns imports required by from that linking couldn't
// satisfy, sorted by import, attribute, and select key.
func (g *Graph) Unresolved(from NodeKey) []UnresolvedImport {
	return slices.Clone(g.unresolved[from])
}

// UnresolvedAll returns every target with unresolved imports, sorted.
func (g *Graph) UnresolvedAll() []NodeKey {
	ks := make([]NodeKey, 0, len(g.unresolved))
	for k, us := range g.unresolved {
		if len(us) > 0 {
			ks = append(ks, k)
		}
	}
	slices.Sort(ks)
	return ks
}

// Embedders returns the targets that directly embed target.
func (g *Graph) Embedders(target NodeKey) []Node {
	var ns []Node
	g.inEdges(target, KindEmbeds, func(r *edgeRec) { ns = append(ns, g.node(r.from)) })
	return ns
}

// EmbeddedBy returns the targets that target embeds, directly or
// transitively.
func (g *Graph) EmbeddedBy(target NodeKey) []Node {
	return g.embedClosure(target, true)
}

// embeddersOf returns the targets that embed target, directly or
// transitively.
func (g *Graph) embeddersOf(target NodeKey) []Node {
	return g.embedClosure(target, false)
}

// IsSelfImport reports whether a dependency from from on to would be a self
// dependency: to is from, or to transitively embeds from (so from's sources
// are compiled into to). This matches resolve.FindResult.IsSelfImport.
func (g *Graph) IsSelfImport(from, to NodeKey) bool {
	if from == to {
		return true
	}
	fi, ok1 := g.ids[from]
	ti, ok2 := g.ids[to]
	if !ok1 || !ok2 {
		return false
	}
	g.embedOnce.Do(g.computeEmbeds)
	_, found := slices.BinarySearch(g.embedded[ti], fi)
	return found
}

func (g *Graph) embedClosure(target NodeKey, out bool) []Node {
	i, ok := g.ids[target]
	if !ok {
		return nil
	}
	g.embedOnce.Do(g.computeEmbeds)
	ids := g.embedders[i]
	if out {
		ids = g.embedded[i]
	}
	ns := make([]Node, len(ids))
	for j, id := range ids {
		ns[j] = g.nodes[id] // IDs are in key order, so ns is sorted
	}
	return ns
}

// computeEmbeds computes transitive Embeds closures for every node with
// Embeds edges.
func (g *Graph) computeEmbeds() {
	g.embedded = make([][]int, len(g.nodes))
	g.embedders = make([][]int, len(g.nodes))
	closure := func(start int, next func(int, func(int))) []int {
		seen := map[int]bool{}
		stack := []int{start}
		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			next(n, func(m int) {
				if !seen[m] {
					seen[m] = true
					stack = append(stack, m)
				}
			})
		}
		delete(seen, start)
		ids := make([]int, 0, len(seen))
		for id := range seen {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		return ids
	}
	outs := func(n int, f func(int)) {
		for _, ei := range g.out[n] {
			if r := &g.edges[ei]; r.kind == KindEmbeds {
				f(g.ids[r.to])
			}
		}
	}
	ins := func(n int, f func(int)) {
		for _, ei := range g.in[n] {
			if r := &g.edges[ei]; r.kind == KindEmbeds {
				f(g.ids[r.from])
			}
		}
	}
	for _, r := range g.edges {
		if r.kind != KindEmbeds {
			continue
		}
		if from := g.ids[r.from]; g.embedded[from] == nil {
			g.embedded[from] = closure(from, outs)
		}
		if to := g.ids[r.to]; g.embedders[to] == nil {
			g.embedders[to] = closure(to, ins)
		}
	}
}

// Outputs returns the generated files target produces.
func (g *Graph) Outputs(target NodeKey) []Node {
	var ns []Node
	g.outEdges(target, KindGenerates, func(r *edgeRec) { ns = append(ns, g.node(r.to)) })
	return ns
}

// Sources returns the files target lists in attribute attr. If attr is
// empty, files in all attributes are returned.
func (g *Graph) Sources(target NodeKey, attr string) []Node {
	var ns []Node
	g.outEdges(target, KindContains, func(r *edgeRec) {
		if attr == "" || r.e.(Contains).Attr == attr {
			ns = append(ns, g.node(r.to))
		}
	})
	return slices.CompactFunc(ns, func(a, b Node) bool { return a.Key == b.Key })
}

// GeneratingRule returns the target that generates file, if any.
func (g *Graph) GeneratingRule(file NodeKey) (Node, bool) {
	var n Node
	var ok bool
	g.inEdges(file, KindGenerates, func(r *edgeRec) { n, ok = g.node(r.from), true })
	return n, ok
}

// Direction selects incoming or outgoing edges.
type Direction int

const (
	Out Direction = iota
	In
)

// ExtensionDep is an extension edge returned by [Graph.ExtensionNeighbors].
type ExtensionDep struct {
	// Node is the node at the other end of the edge.
	Node Node
	Key  string
	Data string
}

// ExtensionNeighbors returns the extension edges of the given kind attached to k,
// sorted by node key and edge key.
//
// kind must be a registered extension kind; for any other kind,
// ExtensionNeighbors returns nil. Core edges are only available through the
// dedicated query methods.
func (g *Graph) ExtensionNeighbors(k NodeKey, kind EdgeKind, dir Direction) []ExtensionDep {
	if _, ok := g.schemas[kind]; !ok || !kind.IsExtension() {
		return nil
	}
	var ds []ExtensionDep
	visit := func(r *edgeRec) {
		other := r.to
		if dir == In {
			other = r.from
		}
		e := r.e.(ExtensionEdge)
		ds = append(ds, ExtensionDep{Node: g.node(other), Key: e.Key, Data: e.Data})
	}
	if dir == In {
		g.inEdges(k, kind, visit)
	} else {
		g.outEdges(k, kind, visit)
	}
	slices.SortFunc(ds, func(a, b ExtensionDep) int {
		return cmp.Or(cmp.Compare(a.Node.Key, b.Node.Key), cmp.Compare(a.Key, b.Key))
	})
	return ds
}

// RelativeLabels converts dependencies to labels relative to from's
// repository and package, sorted by label string and without duplicates.
// Dependencies on imports are skipped. This is a helper for rendering
// attributes in a Materializer.
func RelativeLabels(deps []Dep, from label.Label) []label.Label {
	var ls []label.Label
	for _, d := range deps {
		if d.Node.Kind == KindImport {
			continue
		}
		ls = append(ls, d.Node.Label.Rel(from.Repo, from.Pkg))
	}
	slices.SortFunc(ls, func(a, b label.Label) int { return cmp.Compare(a.String(), b.String()) })
	return slices.CompactFunc(ls, func(a, b label.Label) bool { return a.String() == b.String() })
}
