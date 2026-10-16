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

// Package store implements the graph algorithms the build graph needs:
// strongly connected components and topological sorting. It works on dense
// integer vertex IDs and has no dependencies, so it can be replaced without
// changing the build graph's API.
//
// Vertices are IDs in [0, n). Callers assign IDs in sorted key order, and
// every result here is deterministic.
package store

import (
	"cmp"
	"container/heap"
	"slices"
)

// Arc is a directed edge between two vertex IDs.
type Arc struct{ From, To int64 }

// adjacency returns sorted, deduplicated successor lists.
func adjacency(n int, arcs []Arc) [][]int64 {
	adj := make([][]int64, n)
	for _, a := range arcs {
		adj[a.From] = append(adj[a.From], a.To)
	}
	for i := range adj {
		slices.Sort(adj[i])
		adj[i] = slices.Compact(adj[i])
	}
	return adj
}

// CyclicSCCs returns the strongly connected components that contain a
// cycle: components with more than one vertex, and single vertices with a
// self loop. Each component is sorted by ID, and components are sorted by
// their smallest ID.
//
// It uses an iterative version of Tarjan's algorithm, so deep graphs don't
// overflow the stack.
func CyclicSCCs(n int, arcs []Arc) [][]int64 {
	adj := adjacency(n, arcs)
	const unvisited = -1
	index := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i] = unvisited
	}
	var stack []int64
	next := 0

	type frame struct {
		v    int64
		edge int // next successor to visit
	}
	var out [][]int64
	for root := range int64(n) {
		if index[root] != unvisited {
			continue
		}
		call := []frame{{v: root}}
		index[root], low[root] = next, next
		next++
		stack = append(stack, root)
		onStack[root] = true
		for len(call) > 0 {
			f := &call[len(call)-1]
			if f.edge < len(adj[f.v]) {
				w := adj[f.v][f.edge]
				f.edge++
				switch {
				case index[w] == unvisited:
					index[w], low[w] = next, next
					next++
					stack = append(stack, w)
					onStack[w] = true
					call = append(call, frame{v: w})
				case onStack[w]:
					low[f.v] = min(low[f.v], index[w])
				}
				continue
			}
			// All successors visited: pop the frame.
			v := f.v
			call = call[:len(call)-1]
			if len(call) > 0 {
				p := call[len(call)-1].v
				low[p] = min(low[p], low[v])
			}
			if low[v] != index[v] {
				continue
			}
			var scc []int64
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			if len(scc) > 1 || slices.Contains(adj[v], v) {
				slices.Sort(scc)
				out = append(out, scc)
			}
		}
	}
	slices.SortFunc(out, func(a, b []int64) int { return cmp.Compare(a[0], b[0]) })
	return out
}

// TopoSort returns vertex IDs ordered so that every arc points from an
// earlier vertex to a later one. Among vertices that could come next, the
// smallest ID comes first (Kahn's algorithm with a min-heap). If the graph
// has a cycle, ok is false and the returned order is nil.
func TopoSort(n int, arcs []Arc) (order []int64, ok bool) {
	adj := adjacency(n, arcs)
	indegree := make([]int, n)
	for _, succ := range adj {
		for _, w := range succ {
			indegree[w]++
		}
	}
	ready := &idHeap{}
	for v := range int64(n) {
		if indegree[v] == 0 {
			heap.Push(ready, v)
		}
	}
	order = make([]int64, 0, n)
	for ready.Len() > 0 {
		v := heap.Pop(ready).(int64)
		order = append(order, v)
		for _, w := range adj[v] {
			if indegree[w]--; indegree[w] == 0 {
				heap.Push(ready, w)
			}
		}
	}
	if len(order) != n {
		return nil, false
	}
	return order, true
}

type idHeap []int64

func (h idHeap) Len() int           { return len(h) }
func (h idHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h idHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *idHeap) Push(x any)        { *h = append(*h, x.(int64)) }
func (h *idHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}
