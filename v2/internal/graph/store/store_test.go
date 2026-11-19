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

package store

import (
	"math/rand"
	"slices"
	"testing"
)

// TestCyclicSCCsMatchesReachability compares CyclicSCCs with the definition:
// u and v are in the same cyclic component iff each reaches the other by a
// non-empty path.
func TestCyclicSCCsMatchesReachability(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for iter := range 2000 {
		n := 1 + r.Intn(20)
		var arcs []Arc
		for range r.Intn(3 * n) {
			arcs = append(arcs, Arc{From: int64(r.Intn(n)), To: int64(r.Intn(n))})
		}
		reach := closure(n, arcs)
		want := map[int64][]int64{}
		for u := range n {
			for v := range n {
				if reach[u][v] && reach[v][u] {
					want[int64(u)] = append(want[int64(u)], int64(v))
				}
			}
		}
		got := map[int64][]int64{}
		for _, scc := range CyclicSCCs(n, arcs) {
			if !slices.IsSorted(scc) {
				t.Fatalf("iter %d: unsorted component %v", iter, scc)
			}
			for _, u := range scc {
				got[u] = scc
			}
		}
		for u := range int64(n) {
			if !slices.Equal(want[u], got[u]) {
				t.Fatalf("iter %d: node %d: got component %v, want %v (arcs %v)", iter, u, got[u], want[u], arcs)
			}
		}
		order, ok := TopoSort(n, arcs)
		if ok != (len(want) == 0) {
			t.Fatalf("iter %d: TopoSort ok=%v but cyclic=%v", iter, ok, len(want) > 0)
		}
		if ok {
			pos := make([]int, n)
			for i, id := range order {
				pos[id] = i
			}
			for _, a := range arcs {
				if pos[a.From] >= pos[a.To] {
					t.Fatalf("iter %d: arc %v out of order in %v", iter, a, order)
				}
			}
		}
	}
}

// closure returns reach[u][v] = v is reachable from u by a non-empty path.
func closure(n int, arcs []Arc) [][]bool {
	reach := make([][]bool, n)
	for i := range reach {
		reach[i] = make([]bool, n)
	}
	for _, a := range arcs {
		reach[a.From][a.To] = true
	}
	for k := range n {
		for i := range n {
			for j := range n {
				if reach[i][k] && reach[k][j] {
					reach[i][j] = true
				}
			}
		}
	}
	return reach
}
