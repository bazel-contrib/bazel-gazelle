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
)

// EdgeKind names a type of edge. Core kinds are the constants below.
// Extension-defined kinds are written "<lang>/<name>", where <lang> is the
// name of the extension that registered the kind.
type EdgeKind string

const (
	KindDependsOn EdgeKind = "depends_on"
	KindProvides  EdgeKind = "provides"
	KindRequires  EdgeKind = "requires"
	KindEmbeds    EdgeKind = "embeds"
	KindContains  EdgeKind = "contains"
	KindGenerates EdgeKind = "generates"
	KindAliasOf   EdgeKind = "alias_of"
	KindExports   EdgeKind = "exports"
)

// IsExtension reports whether k is an extension-defined kind.
func (k EdgeKind) IsExtension() bool { return strings.Contains(string(k), "/") }

// Edge is implemented by every edge type: the core types in this package and
// [ExtensionEdge]. The interface has unexported methods, so it can't be
// implemented outside this package; extensions define new kinds of edges
// with ExtensionEdge and an [EdgeSchema].
//
// Edge values are immutable once added to a Builder; slices are copied.
type Edge interface {
	// EdgeKind returns the kind of the edge.
	EdgeKind() EdgeKind

	// discriminator distinguishes edges of the same kind between the same
	// pair of nodes. Edge identity is (from, to, kind, discriminator).
	discriminator() string

	// merge combines two edges with the same identity.
	merge(other Edge) Edge

	// clone returns a deep copy.
	clone() Edge
}

// DependsOn means From needs To at build time through attribute Attr.
// It is the main edge kind considered for cycle detection.
type DependsOn struct {
	// Attr is the attribute holding the dependency, like "deps" or "data".
	Attr string

	// Select is the select() condition key under which the dependency
	// applies, like "@io_bazel_rules_go//go/platform:linux". It's empty for
	// unconditional dependencies.
	Select string

	// Reason lists the imports that justified the edge. It's empty for edges
	// read from build files. Sorted, without duplicates.
	Reason []ImportSpec
}

func (DependsOn) EdgeKind() EdgeKind { return KindDependsOn }

func (e DependsOn) discriminator() string { return e.Attr + "\x00" + e.Select }

func (e DependsOn) merge(other Edge) Edge {
	o := other.(DependsOn)
	e.Reason = normalizeImports(append(slices.Clone(e.Reason), o.Reason...))
	return e
}

func (e DependsOn) clone() Edge {
	e.Reason = slices.Clone(e.Reason)
	return e
}

// Requires means From imports To (an import node). Requires edges are
// produced while collecting facts and consumed by linking: after linking,
// each one has been replaced by DependsOn edges or recorded as unresolved.
// A frozen Graph never contains Requires edges.
type Requires struct {
	// Attr is the attribute the resulting DependsOn edges will use.
	Attr string

	// Select is the select() condition key for the resulting edges.
	Select string
}

func (Requires) EdgeKind() EdgeKind      { return KindRequires }
func (e Requires) discriminator() string { return e.Attr + "\x00" + e.Select }
func (e Requires) merge(Edge) Edge       { return e }
func (e Requires) clone() Edge           { return e }

// Provides means From (a target) can be imported as To (an import node).
type Provides struct{}

func (Provides) EdgeKind() EdgeKind    { return KindProvides }
func (Provides) discriminator() string { return "" }
func (e Provides) merge(Edge) Edge     { return e }
func (e Provides) clone() Edge         { return e }

// Embeds means From embeds To: To's sources are compiled as part of From,
// like go_library's "embed" attribute. A target embedded by another target of
// the same language is not offered as a provider of their common imports.
type Embeds struct{}

func (Embeds) EdgeKind() EdgeKind    { return KindEmbeds }
func (Embeds) discriminator() string { return "" }
func (e Embeds) merge(Edge) Edge     { return e }
func (e Embeds) clone() Edge         { return e }

// Contains means From (a target) lists To (a file) in attribute Attr,
// like "srcs".
type Contains struct {
	Attr string
}

func (Contains) EdgeKind() EdgeKind      { return KindContains }
func (e Contains) discriminator() string { return e.Attr }
func (e Contains) merge(Edge) Edge       { return e }
func (e Contains) clone() Edge           { return e }

// Generates means From (a target) outputs To (a generated file).
type Generates struct{}

func (Generates) EdgeKind() EdgeKind    { return KindGenerates }
func (Generates) discriminator() string { return "" }
func (e Generates) merge(Edge) Edge     { return e }
func (e Generates) clone() Edge         { return e }

// AliasOf means From is an alias for To.
type AliasOf struct{}

func (AliasOf) EdgeKind() EdgeKind    { return KindAliasOf }
func (AliasOf) discriminator() string { return "" }
func (e AliasOf) merge(Edge) Edge     { return e }
func (e AliasOf) clone() Edge         { return e }

// Exports means From re-exports To to its dependents, like java_library's
// "exports" attribute.
type Exports struct{}

func (Exports) EdgeKind() EdgeKind    { return KindExports }
func (Exports) discriminator() string { return "" }
func (e Exports) merge(Edge) Edge     { return e }
func (e Exports) clone() Edge         { return e }

// ExtensionEdge is an extension-defined edge. Its Kind must be registered with an
// [EdgeSchema] before the graph is frozen.
type ExtensionEdge struct {
	// Kind is "<lang>/<name>".
	Kind EdgeKind

	// Key distinguishes multiple edges of the same kind between the same
	// pair of nodes.
	Key string

	// Data is an arbitrary payload. It must be deterministic; it's included
	// in the graph's fingerprint.
	Data string
}

func (e ExtensionEdge) EdgeKind() EdgeKind    { return e.Kind }
func (e ExtensionEdge) discriminator() string { return e.Key }
func (e ExtensionEdge) merge(Edge) Edge       { return e }
func (e ExtensionEdge) clone() Edge           { return e }

// EdgeSchema declares an extension-defined edge kind.
type EdgeSchema struct {
	// Kind is "<lang>/<name>".
	Kind EdgeKind

	// Dependency means edges of this kind are build dependencies: they
	// participate in cycle detection like DependsOn.
	Dependency bool

	// Transitive means [Graph.TransitiveDeps] and [Graph.Reachable] follow
	// edges of this kind.
	Transitive bool
}

func validateSchema(s EdgeSchema) error {
	lang, name, ok := strings.Cut(string(s.Kind), "/")
	if !ok || lang == "" || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("%w: %q: extension edge kinds must be written <lang>/<name>", ErrUnknownEdgeKind, s.Kind)
	}
	return nil
}

func normalizeImports(imps []ImportSpec) []ImportSpec {
	if len(imps) == 0 {
		return nil
	}
	slices.SortFunc(imps, compareImports)
	return slices.Compact(imps)
}

func compareImports(a, b ImportSpec) int {
	return cmp.Or(cmp.Compare(a.Lang, b.Lang), cmp.Compare(a.Imp, b.Imp))
}

// endpointsOK reports whether an edge of kind k may connect nodes of the
// given kinds.
func endpointsOK(k EdgeKind, from, to NodeKind) bool {
	isTarget := func(n NodeKind) bool { return n == KindTarget || n == KindExternal }
	isFile := func(n NodeKind) bool { return n == KindSourceFile || n == KindGeneratedFile }
	switch k {
	case KindDependsOn:
		return from == KindTarget && (isTarget(to) || isFile(to))
	case KindRequires, KindProvides:
		return isTarget(from) && to == KindImport
	case KindEmbeds, KindAliasOf, KindExports:
		return from == KindTarget && isTarget(to)
	case KindContains:
		return from == KindTarget && isFile(to)
	case KindGenerates:
		return from == KindTarget && to == KindGeneratedFile
	default:
		// Extension edges may connect any nodes except imports, which only
		// core edges understand.
		return from != KindImport && to != KindImport
	}
}
