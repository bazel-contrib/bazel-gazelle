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
	"slices"
	"sync"
)

// Graph is an immutable build graph. All methods are safe for concurrent use.
// All results are deterministic: unless documented otherwise, they are sorted
// by node key, and returned slices are fresh copies the caller may modify.
//
// Graph deliberately has no method to enumerate raw edges. Extensions ask
// questions through the query methods; new kinds of questions should become
// new methods here so all languages share one implementation.
type Graph struct {
	// nodes is sorted by key. A node's index is its ID.
	nodes []Node
	ids   map[NodeKey]int

	// edges is sorted by (from, kind, to, discriminator).
	edges []edgeRec

	// out and in hold edge indices per node ID, in edges order (out) or
	// sorted by (from, kind, discriminator) (in).
	out, in [][]int

	unresolved map[NodeKey][]UnresolvedImport
	schemas    map[EdgeKind]EdgeSchema

	cyclesOnce sync.Once
	cycles     []Cycle

	// embedOnce guards the transitive closures of Embeds edges, computed
	// once since linking queries them for every required import. Each list
	// holds sorted node IDs.
	embedOnce sync.Once
	embedded  [][]int // node ID -> IDs it embeds, transitively
	embedders [][]int // node ID -> IDs embedding it, transitively
}

func newGraph(b *Builder) *Graph {
	g := &Graph{
		ids:        make(map[NodeKey]int, len(b.nodes)),
		unresolved: make(map[NodeKey][]UnresolvedImport, len(b.unresolved)),
		schemas:    b.schemas,
	}
	g.nodes = make([]Node, 0, len(b.nodes))
	for _, n := range b.nodes {
		g.nodes = append(g.nodes, *n)
	}
	slices.SortFunc(g.nodes, func(a, b Node) int { return cmp.Compare(a.Key, b.Key) })
	for i, n := range g.nodes {
		g.ids[n.Key] = i
	}

	g.edges = make([]edgeRec, 0, len(b.edges))
	for _, r := range b.edges {
		g.edges = append(g.edges, *r)
	}
	slices.SortFunc(g.edges, func(a, b edgeRec) int { return compareEdgeIDs(a.edgeID, b.edgeID) })
	g.out = make([][]int, len(g.nodes))
	g.in = make([][]int, len(g.nodes))
	for i, r := range g.edges {
		g.out[g.ids[r.from]] = append(g.out[g.ids[r.from]], i)
		g.in[g.ids[r.to]] = append(g.in[g.ids[r.to]], i)
	}
	for _, in := range g.in {
		slices.SortFunc(in, func(a, b int) int {
			ea, eb := g.edges[a], g.edges[b]
			return cmp.Or(
				cmp.Compare(ea.from, eb.from),
				cmp.Compare(ea.kind, eb.kind),
				cmp.Compare(ea.disc, eb.disc))
		})
	}

	for k, us := range b.unresolved {
		us = slices.Clone(us)
		slices.SortFunc(us, func(a, b UnresolvedImport) int {
			return cmp.Or(compareImports(a.Import, b.Import), cmp.Compare(a.Attr, b.Attr), cmp.Compare(a.Select, b.Select))
		})
		g.unresolved[k] = us
	}
	return g
}

func compareEdgeIDs(a, b edgeID) int {
	return cmp.Or(
		cmp.Compare(a.from, b.from),
		cmp.Compare(a.kind, b.kind),
		cmp.Compare(a.to, b.to),
		cmp.Compare(a.disc, b.disc))
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int { return len(g.nodes) }

// Node returns the node with the given key.
func (g *Graph) Node(k NodeKey) (Node, bool) {
	i, ok := g.ids[k]
	if !ok {
		return Node{}, false
	}
	return g.nodes[i], true
}

// TargetsInPackage returns the targets in package rel of the main
// repository.
func (g *Graph) TargetsInPackage(rel string) []Node {
	var ns []Node
	for _, n := range g.nodes {
		if n.Kind == KindTarget && n.Label.Pkg == rel {
			ns = append(ns, n)
		}
	}
	return ns
}

// outEdges calls f for each out-edge of k with the given kind, in order.
func (g *Graph) outEdges(k NodeKey, kind EdgeKind, f func(r *edgeRec)) {
	i, ok := g.ids[k]
	if !ok {
		return
	}
	for _, ei := range g.out[i] {
		if r := &g.edges[ei]; r.kind == kind {
			f(r)
		}
	}
}

// inEdges calls f for each in-edge of k with the given kind, in order.
func (g *Graph) inEdges(k NodeKey, kind EdgeKind, f func(r *edgeRec)) {
	i, ok := g.ids[k]
	if !ok {
		return
	}
	for _, ei := range g.in[i] {
		if r := &g.edges[ei]; r.kind == kind {
			f(r)
		}
	}
}

func (g *Graph) node(k NodeKey) Node { return g.nodes[g.ids[k]] }
