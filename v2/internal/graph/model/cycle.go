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
	"fmt"
	"slices"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/store"
)

// Cycle is a group of targets that depend on each other: a strongly
// connected component of the dependency subgraph.
type Cycle struct {
	// Nodes are the members of the component, sorted.
	Nodes []NodeKey

	// Path is one shortest cycle through the component, starting and ending
	// at Nodes[0]. It is meant for error messages.
	Path []CycleEdge

	// Select is empty if the cycle exists among unconditional dependencies.
	// Otherwise, it's the select() key under which the cycle exists.
	Select string
}

// CycleEdge is one step of a [Cycle] path.
type CycleEdge struct {
	From, To NodeKey
	Kind     EdgeKind
	Attr     string
	Select   string
	Reason   []ImportSpec
	Origin   Origin

	// Via is set when the dependency goes through a generated file: From
	// depends on Via, which To generates.
	Via NodeKey
}

// String formats the cycle for error messages, one edge per line.
func (c Cycle) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "dependency cycle among %d target(s)", len(c.Nodes))
	if c.Select != "" {
		fmt.Fprintf(&sb, " when %s is selected", c.Select)
	}
	sb.WriteString(":")
	for _, e := range c.Path {
		fmt.Fprintf(&sb, "\n  %s --", keyLabel(e.From))
		if e.Kind == KindDependsOn {
			sb.WriteString(e.Attr)
		} else {
			sb.WriteString(string(e.Kind))
		}
		var notes []string
		for _, imp := range e.Reason {
			notes = append(notes, fmt.Sprintf("imports %q", imp.Imp))
		}
		if e.Via != "" {
			notes = append(notes, "via "+keyLabel(e.Via))
		}
		if len(e.Reason) == 0 && e.Origin != 0 {
			notes = append(notes, e.Origin.String())
		}
		if len(notes) > 0 {
			fmt.Fprintf(&sb, " [%s]", strings.Join(notes, ", "))
		}
		fmt.Fprintf(&sb, "--> %s", keyLabel(e.To))
	}
	return sb.String()
}

// keyLabel formats a key's label the way users write it.
func keyLabel(k NodeKey) string {
	info, err := ParseKey(k)
	if err != nil || info.Class == KindImport {
		return string(k)
	}
	return info.Label.String()
}

// CycleError reports a dependency cycle. It wraps ErrCycle.
type CycleError struct {
	Cycle Cycle
}

func (e *CycleError) Error() string { return e.Cycle.String() }
func (e *CycleError) Unwrap() error { return ErrCycle }

type cycleArc struct {
	from, to int // dense IDs among targets, assigned by dependencyArcs
	sel      string
	edge     CycleEdge
}

// dependencyArcs returns the arcs of the dependency subgraph in a
// deterministic order, with vertices numbered densely (keys[i] is vertex i).
//
// The subgraph has DependsOn, Exports, and AliasOf edges and extension edges
// whose schema sets Dependency. Edges from targets to generated files,
// including Contains edges (a generated file in srcs), are lowered to edges
// to the generating rule. Edges to source files are ignored. If filter is
// non-nil, DependsOn edges must also match it.
func (g *Graph) dependencyArcs(filter *DepQuery) (arcs []cycleArc, keys []NodeKey) {
	ids := map[NodeKey]int{}
	id := func(k NodeKey) int {
		if i, ok := ids[k]; ok {
			return i
		}
		ids[k] = len(keys)
		keys = append(keys, k)
		return ids[k]
	}
	for i := range g.edges {
		r := &g.edges[i]
		if g.node(r.from).Kind != KindTarget {
			continue
		}
		ce := CycleEdge{From: r.from, To: r.to, Kind: r.kind, Origin: r.origin}
		switch r.kind {
		case KindDependsOn:
			if filter != nil && !filter.matchEdge(r) {
				continue
			}
			d := r.e.(DependsOn)
			ce.Attr, ce.Select, ce.Reason = d.Attr, d.Select, slices.Clone(d.Reason)
		case KindContains:
			ce.Attr = r.e.(Contains).Attr
		case KindExports, KindAliasOf:
		default:
			if s, ok := g.schemas[r.kind]; !ok || !s.Dependency {
				continue
			}
		}
		switch to := g.node(r.to); to.Kind {
		case KindGeneratedFile:
			gen, ok := g.GeneratingRule(to.Key)
			if !ok {
				continue
			}
			ce.Via, ce.To = to.Key, gen.Key
		case KindTarget, KindExternal:
			if r.kind == KindContains {
				continue
			}
		default:
			continue
		}
		arcs = append(arcs, cycleArc{sel: ce.Select, edge: ce})
	}
	// Number vertices in key order so results don't depend on edge order.
	var all []NodeKey
	for _, a := range arcs {
		all = append(all, a.edge.From, a.edge.To)
	}
	slices.Sort(all)
	for _, k := range slices.Compact(all) {
		id(k)
	}
	for i := range arcs {
		arcs[i].from, arcs[i].to = ids[arcs[i].edge.From], ids[arcs[i].edge.To]
	}
	slices.SortStableFunc(arcs, func(a, b cycleArc) int {
		return cmp.Or(cmp.Compare(a.from, b.from), cmp.Compare(a.to, b.to))
	})
	return arcs, keys
}

// Cycles returns the dependency cycles in the graph, sorted by their
// smallest node key, then select key.
//
// The dependency subgraph is described in the package documentation: it
// contains DependsOn, Exports, and AliasOf edges between targets, and
// extension edges whose schema sets Dependency. A dependency on a generated
// file (including listing it in srcs) counts as a dependency on the rule
// that generates it.
//
// Edges in different select() branches are mutually exclusive. Cycles are
// computed once for unconditional edges, then once per select key for
// unconditional edges plus that key's edges. A component is reported once,
// with the first select key (in that order) under which it is cyclic. For a
// component found under a select key, Path uses at least one edge of that
// key.
func (g *Graph) Cycles() []Cycle {
	g.cyclesOnce.Do(func() { g.cycles = g.computeCycles() })
	cs := make([]Cycle, len(g.cycles))
	for i, c := range g.cycles {
		cs[i] = cloneCycle(c)
	}
	return cs
}

func cloneCycle(c Cycle) Cycle {
	c.Nodes = slices.Clone(c.Nodes)
	c.Path = slices.Clone(c.Path)
	for i := range c.Path {
		c.Path[i].Reason = slices.Clone(c.Path[i].Reason)
	}
	return c
}

func (g *Graph) computeCycles() []Cycle {
	arcs, keys := g.dependencyArcs(nil)

	// Every cycle under any select key lies within one strongly connected
	// component of the graph with all edges. Find those first, and keep only
	// arcs inside them, so the per-key passes below are cheap.
	component := make(map[int]int)
	for i, scc := range store.CyclicSCCs(len(keys), storeArcs(arcs)) {
		for _, id := range scc {
			component[int(id)] = i
		}
	}
	inner := arcs[:0:0]
	for _, a := range arcs {
		c1, ok1 := component[a.from]
		c2, ok2 := component[a.to]
		if ok1 && ok2 && c1 == c2 {
			inner = append(inner, a)
		}
	}
	arcs = inner

	var sels []string
	for _, a := range arcs {
		if a.sel != "" {
			sels = append(sels, a.sel)
		}
	}
	slices.Sort(sels)
	sels = slices.Compact(sels)

	var cycles []Cycle
	seen := map[string]bool{}
	for _, sel := range append([]string{""}, sels...) {
		sub := subgraph(arcs, sel)
		for _, scc := range store.CyclicSCCs(len(keys), storeArcs(sub)) {
			members := make([]NodeKey, len(scc))
			for i, id := range scc {
				members[i] = keys[id]
			}
			id := strings.Join(slicesString(members), "\x00")
			if seen[id] {
				continue
			}
			seen[id] = true
			cycles = append(cycles, Cycle{Nodes: members, Path: cyclePath(scc, sub, sel), Select: sel})
		}
	}
	slices.SortFunc(cycles, func(a, b Cycle) int {
		return cmp.Or(cmp.Compare(a.Nodes[0], b.Nodes[0]), cmp.Compare(a.Select, b.Select))
	})
	return cycles
}

// subgraph returns unconditional arcs plus arcs under sel.
func subgraph(arcs []cycleArc, sel string) []cycleArc {
	var sub []cycleArc
	for _, a := range arcs {
		if a.sel == "" || a.sel == sel {
			sub = append(sub, a)
		}
	}
	return sub
}

func storeArcs(arcs []cycleArc) []store.Arc {
	sa := make([]store.Arc, len(arcs))
	for i, a := range arcs {
		sa[i] = store.Arc{From: int64(a.from), To: int64(a.to)}
	}
	return sa
}

func slicesString(ks []NodeKey) []string {
	ss := make([]string, len(ks))
	for i, k := range ks {
		ss[i] = string(k)
	}
	return ss
}

// cyclePath returns a shortest cycle within the component. For sel == "",
// it's a shortest cycle through the component's smallest vertex. Otherwise,
// it's the shortest cycle using an arc under sel (ties broken by arc order),
// since a cycle of only unconditional arcs would have been reported without
// a select key.
func cyclePath(scc []int64, arcs []cycleArc, sel string) []CycleEdge {
	in := map[int]bool{}
	for _, id := range scc {
		in[int(id)] = true
	}
	adj := map[int][]int{}
	for i, a := range arcs {
		if in[a.from] && in[a.to] {
			adj[a.from] = append(adj[a.from], i)
		}
	}
	// path returns arc indices of a shortest path from start to end
	// (non-empty when start == end).
	path := func(start, end int) []int {
		parent := map[int]int{}
		queue := []int{start}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			for _, ai := range adj[n] {
				to := arcs[ai].to
				if to == end {
					p := []int{ai}
					for c := n; c != start; c = arcs[parent[c]].from {
						p = append(p, parent[c])
					}
					slices.Reverse(p)
					return p
				}
				if _, ok := parent[to]; !ok && to != start {
					parent[to] = ai
					queue = append(queue, to)
				}
			}
		}
		return nil
	}
	var best []int
	if sel == "" {
		best = path(int(scc[0]), int(scc[0]))
	} else {
		for i, a := range arcs {
			if a.sel != sel || !in[a.from] || !in[a.to] {
				continue
			}
			var p []int
			if a.from == a.to {
				p = []int{i}
			} else if rest := path(a.to, a.from); rest != nil {
				p = append([]int{i}, rest...)
			}
			if p != nil && (best == nil || len(p) < len(best)) {
				best = p
			}
		}
	}
	edges := make([]CycleEdge, len(best))
	for i, ai := range best {
		edges[i] = arcs[ai].edge
	}
	return edges
}

// TopoOrder returns targets ordered so that every target comes after the
// targets it depends on. It considers the same edges as [Graph.Cycles]
// (including generated-file lowering), with DependsOn edges filtered by q.
// Unlike Cycles, it doesn't separate select() branches: use q.Select to
// order one configuration. Ties are broken by key. Targets with no
// dependency edges are omitted.
//
// If the considered edges form a cycle, TopoOrder returns a *CycleError
// describing one.
func (g *Graph) TopoOrder(q DepQuery) ([]NodeKey, error) {
	arcs, keys := g.dependencyArcs(&q)
	rev := make([]store.Arc, len(arcs))
	for i, a := range arcs {
		// Reverse arcs so dependencies sort first.
		rev[i] = store.Arc{From: int64(a.to), To: int64(a.from)}
	}
	order, ok := store.TopoSort(len(keys), rev)
	if !ok {
		sccs := store.CyclicSCCs(len(keys), storeArcs(arcs))
		members := make([]NodeKey, len(sccs[0]))
		for i, id := range sccs[0] {
			members[i] = keys[id]
		}
		return nil, &CycleError{Cycle: Cycle{Nodes: members, Path: cyclePath(sccs[0], arcs, "")}}
	}
	ks := make([]NodeKey, len(order))
	for i, id := range order {
		ks[i] = keys[id]
	}
	return ks, nil
}
