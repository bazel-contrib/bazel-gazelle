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
	"errors"
	"fmt"
	"maps"
	"slices"
)

// BuilderOptions configures a Builder.
type BuilderOptions struct {
	// Schemas declares extension-defined edge kinds. Extension edges of other
	// kinds are rejected.
	Schemas []EdgeSchema
}

// Builder accumulates facts about the build and produces an immutable
// [Graph] with Freeze. A Builder is not safe for concurrent use.
//
// Adding the same node or edge twice is not an error: the facts are merged.
// This lets facts from existing build files and from generation be combined
// in any order with the same result.
//
// Extensions never receive a Builder; they receive a package-scoped [Facts]
// sink from Gazelle.
type Builder struct {
	nodes      map[NodeKey]*Node
	edges      map[edgeID]*edgeRec
	unresolved map[NodeKey][]UnresolvedImport
	schemas    map[EdgeKind]EdgeSchema
	frozen     bool

	// warnings records non-fatal conflicts, like two facts giving a node
	// different rule kinds.
	warnings map[string]bool

	// ranks holds the highest originRank of explicit facts for each node.
	ranks map[NodeKey]int
}

type edgeID struct {
	from, to NodeKey
	kind     EdgeKind
	disc     string
}

type edgeRec struct {
	edgeID
	e      Edge
	origin Origin
}

// UnresolvedImport is a Requires fact that linking couldn't satisfy.
type UnresolvedImport struct {
	Import ImportSpec
	Attr   string
	Select string
	// Err describes why the import couldn't be resolved. It wraps
	// ErrUnresolvedImport or ErrAmbiguousImport.
	Err error
}

// NewBuilder returns an empty Builder.
func NewBuilder(opts BuilderOptions) (*Builder, error) {
	b := &Builder{
		nodes:      make(map[NodeKey]*Node),
		edges:      make(map[edgeID]*edgeRec),
		unresolved: make(map[NodeKey][]UnresolvedImport),
		schemas:    make(map[EdgeKind]EdgeSchema),
		ranks:      make(map[NodeKey]int),
		warnings:   make(map[string]bool),
	}
	var errs []error
	for _, s := range opts.Schemas {
		if err := validateSchema(s); err != nil {
			errs = append(errs, err)
			continue
		}
		b.schemas[s.Kind] = s
	}
	return b, errors.Join(errs...)
}

// AddNode adds a node or merges it with an existing node with the same key.
//
// Kind, Label, and Import are derived from the key; if n.Kind is set, it must
// agree.
//
// When facts give a node different RuleKind or Lang values, the result
// doesn't depend on the order facts were added: the value from the fact
// with the highest-precedence origin wins (generated, then existing, then
// indexed, then others), and among equal origins the smaller value wins.
// Conflicts are reported by [Builder.Warnings].
func (b *Builder) AddNode(n Node) error {
	if b.frozen {
		return ErrFrozen
	}
	kn, err := nodeFromKey(n.Key)
	if err != nil {
		return err
	}
	if n.Kind != 0 && n.Kind != kn.Kind {
		return fmt.Errorf("%w: %s: key implies kind %s, got %s", ErrConflictingNode, n.Key, kn.Kind, n.Kind)
	}
	kn.RuleKind, kn.Lang, kn.Origin = n.RuleKind, n.Lang, n.Origin
	old, ok := b.nodes[n.Key]
	if !ok {
		b.nodes[n.Key] = &kn
		b.setRank(n.Key, kn.Origin)
		return nil
	}
	oldRank, newRank := b.ranks[n.Key], originRank(kn.Origin)
	old.RuleKind = b.pick(n.Key, "rule kind", old.RuleKind, oldRank, kn.RuleKind, newRank)
	old.Lang = b.pick(n.Key, "language", old.Lang, oldRank, kn.Lang, newRank)
	old.Origin |= kn.Origin
	b.setRank(n.Key, kn.Origin)
	return nil
}

// originRank orders origins for conflict resolution; higher wins.
func originRank(o Origin) int {
	switch {
	case o&OriginGenerated != 0:
		return 3
	case o&OriginExisting != 0:
		return 2
	case o&OriginIndexed != 0:
		return 1
	default:
		return 0
	}
}

func (b *Builder) setRank(k NodeKey, o Origin) {
	if r := originRank(o); r > b.ranks[k] {
		b.ranks[k] = r
	}
}

// pick chooses between two values of a node field deterministically.
func (b *Builder) pick(k NodeKey, field, oldV string, oldRank int, newV string, newRank int) string {
	switch {
	case newV == "" || newV == oldV:
		return oldV
	case oldV == "":
		return newV
	}
	b.warnings[fmt.Sprintf("%s: conflicting %s %q and %q", k, field, oldV, newV)] = true
	if newRank != oldRank {
		if newRank > oldRank {
			return newV
		}
		return oldV
	}
	return min(oldV, newV)
}

// Warnings returns non-fatal conflicts found while adding facts, sorted.
func (b *Builder) Warnings() []string {
	ws := slices.Collect(maps.Keys(b.warnings))
	slices.Sort(ws)
	return ws
}

// ensureNode adds a node implied by its key if it doesn't exist yet.
func (b *Builder) ensureNode(k NodeKey, o Origin) (*Node, error) {
	if n, ok := b.nodes[k]; ok {
		return n, nil
	}
	n, err := nodeFromKey(k)
	if err != nil {
		return nil, err
	}
	n.Origin = o
	b.nodes[k] = &n
	return &n, nil
}

// AddEdge adds an edge or merges it with an existing edge with the same
// identity. Missing endpoint nodes are created with kinds implied by their
// keys.
func (b *Builder) AddEdge(from, to NodeKey, e Edge, o Origin) error {
	if b.frozen {
		return ErrFrozen
	}
	if e == nil {
		return fmt.Errorf("%w: nil edge from %s to %s", ErrInvalidEdge, from, to)
	}
	kind := e.EdgeKind()
	if kind.IsExtension() {
		if _, ok := b.schemas[kind]; !ok {
			return fmt.Errorf("%w: %q (from %s to %s)", ErrUnknownEdgeKind, kind, from, to)
		}
	} else if _, ok := e.(ExtensionEdge); ok {
		return fmt.Errorf("%w: %q: extension edge kinds must be written <lang>/<name>", ErrUnknownEdgeKind, kind)
	}
	// Validate before adding anything, so a rejected edge leaves no trace.
	fn, err := nodeFromKey(from)
	if err != nil {
		return err
	}
	tn, err := nodeFromKey(to)
	if err != nil {
		return err
	}
	if !endpointsOK(kind, fn.Kind, tn.Kind) {
		return fmt.Errorf("%w: %s edge may not connect %s %s to %s %s", ErrInvalidEdge, kind, fn.Kind, from, tn.Kind, to)
	}
	if _, err := b.ensureNode(from, o); err != nil {
		return err
	}
	if _, err := b.ensureNode(to, o); err != nil {
		return err
	}
	id := edgeID{from: from, to: to, kind: kind, disc: e.discriminator()}
	if old, ok := b.edges[id]; ok {
		old.e = old.e.merge(e)
		old.origin |= o
		return nil
	}
	// Merging with an empty value normalizes payloads (sorts Reason).
	b.edges[id] = &edgeRec{edgeID: id, e: e.clone().merge(e.clone()), origin: o}
	return nil
}

// RequireFact is a pending Requires edge, returned by PendingRequires.
type RequireFact struct {
	From   NodeKey
	Import ImportSpec
	Requires
}

// PendingRequires returns the Requires edges that haven't been linked yet,
// sorted by origin node, import, attribute, and select key.
//
// This is used by Gazelle's link step; extensions don't call it.
func (b *Builder) PendingRequires() []RequireFact {
	var facts []RequireFact
	for id, r := range b.edges {
		if id.kind != KindRequires {
			continue
		}
		facts = append(facts, RequireFact{
			From:     id.from,
			Import:   b.nodes[id.to].Import,
			Requires: r.e.(Requires),
		})
	}
	slices.SortFunc(facts, func(a, b RequireFact) int {
		return cmp.Or(
			cmp.Compare(a.From, b.From),
			compareImports(a.Import, b.Import),
			cmp.Compare(a.Attr, b.Attr),
			cmp.Compare(a.Select, b.Select))
	})
	return facts
}

// Link replaces a pending Requires edge with DependsOn edges to each of to.
// If to is empty, the dependency is dropped (for example, because the import
// is provided by the standard library). Self edges are dropped.
//
// This is used by Gazelle's link step; extensions don't call it.
func (b *Builder) Link(f RequireFact, to []NodeKey, o Origin) error {
	if b.frozen {
		return ErrFrozen
	}
	if _, ok := b.edges[f.id()]; !ok {
		return fmt.Errorf("%w: no pending requires edge from %s to %s", ErrUnlinked, f.From, ImportKey(f.Import))
	}
	// Validate all edges first so a failure leaves the fact pending.
	dep := DependsOn{Attr: f.Attr, Select: f.Select, Reason: []ImportSpec{f.Import}}
	for _, t := range to {
		tn, err := nodeFromKey(t)
		if err != nil {
			return err
		}
		if t != f.From && !endpointsOK(KindDependsOn, KindTarget, tn.Kind) {
			return fmt.Errorf("%w: cannot link %s to %s %s", ErrInvalidEdge, f.From, tn.Kind, t)
		}
	}
	for _, t := range to {
		if t != f.From {
			if err := b.AddEdge(f.From, t, dep, o); err != nil {
				return err
			}
		}
	}
	return b.dropRequires(f)
}

// MarkUnresolved replaces a pending Requires edge with an unresolved import
// record, queryable with [Graph.Unresolved]. If err doesn't wrap
// ErrUnresolvedImport or ErrAmbiguousImport, it is wrapped with
// ErrUnresolvedImport.
//
// This is used by Gazelle's link step; extensions don't call it.
func (b *Builder) MarkUnresolved(f RequireFact, err error) error {
	if b.frozen {
		return ErrFrozen
	}
	if dropErr := b.dropRequires(f); dropErr != nil {
		return dropErr
	}
	if !errors.Is(err, ErrUnresolvedImport) && !errors.Is(err, ErrAmbiguousImport) {
		err = fmt.Errorf("%w: %v", ErrUnresolvedImport, err)
	}
	b.unresolved[f.From] = append(b.unresolved[f.From], UnresolvedImport{
		Import: f.Import, Attr: f.Attr, Select: f.Select, Err: err,
	})
	return nil
}

func (f RequireFact) id() edgeID {
	return edgeID{from: f.From, to: ImportKey(f.Import), kind: KindRequires, disc: f.Requires.discriminator()}
}

func (b *Builder) dropRequires(f RequireFact) error {
	id := f.id()
	if _, ok := b.edges[id]; !ok {
		return fmt.Errorf("%w: no pending requires edge from %s to %s", ErrUnlinked, f.From, id.to)
	}
	delete(b.edges, id)
	return nil
}

// Node returns the node with key k, if it has been added.
func (b *Builder) Node(k NodeKey) (Node, bool) {
	n, ok := b.nodes[k]
	if !ok {
		return Node{}, false
	}
	return *n, true
}

// Snapshot returns a read-only Graph of the facts added so far, without
// validating them. Pending Requires edges are included but no query returns
// them. Later changes to the Builder don't affect the snapshot.
//
// Gazelle's link step uses a snapshot to query providers and embeds before
// the graph is complete.
func (b *Builder) Snapshot() *Graph {
	snap := &Builder{
		nodes:      make(map[NodeKey]*Node, len(b.nodes)),
		edges:      make(map[edgeID]*edgeRec, len(b.edges)),
		unresolved: b.unresolved,
		schemas:    b.schemas,
	}
	for k, n := range b.nodes {
		n := *n
		snap.nodes[k] = &n
	}
	for id, r := range b.edges {
		snap.edges[id] = &edgeRec{edgeID: id, e: r.e.clone(), origin: r.origin}
		for _, k := range []NodeKey{id.from, id.to} {
			if _, ok := snap.nodes[k]; !ok {
				if n, err := nodeFromKey(k); err == nil {
					snap.nodes[k] = &n
				}
			}
		}
	}
	return newGraph(snap)
}

// Freeze validates the facts and returns an immutable Graph. The Builder
// can't be used afterward.
//
// Freeze fails if Requires edges remain or if a generated file has more than
// one generating rule. Dependency cycles are not a Freeze error; see
// [Graph.Cycles].
func (b *Builder) Freeze() (*Graph, error) {
	if b.frozen {
		return nil, ErrFrozen
	}
	b.frozen = true

	var errs []error
	generators := make(map[NodeKey][]NodeKey)
	for id := range b.edges {
		switch id.kind {
		case KindRequires:
			errs = append(errs, fmt.Errorf("%w: %s requires %s", ErrUnlinked, id.from, id.to))
		case KindGenerates:
			generators[id.to] = append(generators[id.to], id.from)
		}
	}
	for file, gens := range generators {
		if len(gens) > 1 {
			slices.Sort(gens)
			errs = append(errs, fmt.Errorf("%w: %s is generated by %v", ErrDuplicateOutput, file, gens))
		}
	}
	if len(errs) > 0 {
		slices.SortFunc(errs, func(a, b error) int { return cmp.Compare(a.Error(), b.Error()) })
		return nil, errors.Join(errs...)
	}
	return newGraph(b), nil
}
