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
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/label"
)

// NodeKind classifies vertices.
type NodeKind uint8

const (
	// KindTarget is a rule target in the main repository.
	KindTarget NodeKind = iota + 1

	// KindExternal is a target in another repository. It is a leaf: Gazelle
	// doesn't know its dependencies.
	KindExternal

	// KindSourceFile is a checked-in file.
	KindSourceFile

	// KindGeneratedFile is a file produced by a rule.
	KindGeneratedFile

	// KindImport is an importable name in a language namespace. It is not a
	// Bazel entity; it models what targets provide and require.
	KindImport
)

func (k NodeKind) String() string {
	switch k {
	case KindTarget:
		return "target"
	case KindExternal:
		return "external"
	case KindSourceFile:
		return "source_file"
	case KindGeneratedFile:
		return "generated_file"
	case KindImport:
		return "import"
	default:
		return "unknown"
	}
}

// Origin records where a fact came from. It is a bit set: a node or edge
// contributed by several sources has all of their bits. Origin is used for
// merge precedence and diagnostics. It is not part of identity, and it is not
// part of the graph fingerprint.
type Origin uint8

const (
	// OriginExisting means the fact was read from an existing build file in a
	// directory Gazelle is updating.
	OriginExisting Origin = 1 << iota

	// OriginGenerated means the fact was produced by an extension in this run.
	OriginGenerated

	// OriginIndexed means the fact was read from a build file in a directory
	// Gazelle indexed but didn't update.
	OriginIndexed

	// OriginExternal means the fact was synthesized for an external label.
	OriginExternal

	// OriginKeep means the fact came from a rule or attribute value marked
	// with a "# keep" comment. Gazelle never removes kept facts.
	OriginKeep
)

func (o Origin) String() string {
	if o == 0 {
		return "none"
	}
	var parts []string
	for _, b := range []struct {
		o Origin
		s string
	}{
		{OriginExisting, "existing"},
		{OriginGenerated, "generated"},
		{OriginIndexed, "indexed"},
		{OriginExternal, "external"},
		{OriginKeep, "keep"},
	} {
		if o&b.o != 0 {
			parts = append(parts, b.s)
		}
	}
	return strings.Join(parts, "|")
}

// Node is a vertex. Nodes are values; modifying a returned Node doesn't
// affect the graph.
type Node struct {
	Key  NodeKey
	Kind NodeKind

	// Label is the entity's label. It is the zero value for KindImport.
	Label label.Label

	// Import is set for KindImport.
	Import ImportSpec

	// RuleKind is the kind of rule for targets, like "go_library". It's empty
	// for other nodes and for targets whose kind isn't known (for example,
	// external targets).
	RuleKind string

	// Lang is the name of the extension that owns the node, if known.
	Lang string

	// Origin is the union of the origins of all facts that contributed this
	// node.
	Origin Origin
}

// nodeFromKey creates a node with fields implied by the key.
func nodeFromKey(k NodeKey) (Node, error) {
	info, err := ParseKey(k)
	if err != nil {
		return Node{}, err
	}
	n := Node{Key: k, Kind: info.Class, Label: info.Label, Import: info.Import}
	if info.Class == KindTarget && !info.IsMainRepo() {
		n.Kind = KindExternal
	}
	return n, nil
}
