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

package graph

import (
	"context"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/bazel-contrib/bazel-gazelle/v2/walk"
)

// The interfaces below are optional and implemented by language extensions
// (the value returned by NewV2). They are used only with
// -experimental_build_graph. An extension implementing Materializer should
// also implement resolve.Resolver so it keeps working without the flag and
// with Gazelle v1.

// SchemaDeclarer declares how an extension participates in the build graph.
// DeclareGraphSchema is called once, after OnStart.
type SchemaDeclarer interface {
	DeclareGraphSchema(context.Context) (SchemaResult, error)
}

// SchemaResult is returned by SchemaDeclarer.DeclareGraphSchema.
type SchemaResult struct {
	// Edges declares extension-defined edge kinds. Each kind must be named
	// "<lang>/<name>", where <lang> is the extension's name.
	Edges []EdgeSchema

	// AcceptProvidersFrom lists other extensions whose providers may satisfy
	// imports required by this extension's rules, in order of preference.
	// Providers from this extension are always accepted and preferred.
	AcceptProvidersFrom []string
}

// Contributor adds facts about rules the extension generated. It's called
// once per updated directory, after generated rules are merged into the
// build file. Gazelle has already added a node for every rule in the file,
// edges for label-valued attributes it recognizes, and Provides and Embeds
// edges from resolve.Indexer. Contributors add what Gazelle can't infer,
// most importantly the imports each rule requires.
//
// Contributor is ignored (with a warning) unless the extension also
// implements Materializer.
type Contributor interface {
	ContributeGraph(context.Context, ContributeArgs) error
}

// ContributeArgs are the arguments to Contributor.ContributeGraph.
type ContributeArgs struct {
	// Config is the configuration for the directory.
	Config *config.Config

	// Rel is the slash-separated path to the directory, relative to the
	// repository root.
	Rel string

	// File is the build file after generated rules were merged into it.
	File *rule.File

	// Gen lists the rules this extension generated, in the order of
	// GenerateResult.Gen. Imports is aligned with Gen and holds the values
	// from GenerateResult.Imports (nil elements if none were returned).
	Gen     []*rule.Rule
	Imports []any

	// Nodes is aligned with Gen and holds each generated rule's key. A
	// generated rule merged into an existing rule with a different name gets
	// the existing rule's key. A key is empty if the rule couldn't be merged
	// or was merged into a rule marked "# keep".
	Nodes []NodeKey

	// Facts receives facts about targets in this directory.
	Facts Facts

	// Cache is an in-memory cache of file metadata.
	Cache *walk.Cache
}

// Materializer sets a generated rule's dependency attributes from the frozen
// graph. It replaces resolve.Resolver in graph mode. Materialize must not
// resolve imports itself: every dependency it writes should come from a
// graph query, so the graph Gazelle validated is the graph that is written.
type Materializer interface {
	Materialize(context.Context, MaterializeArgs) error
}

// MaterializeArgs are the arguments to Materializer.Materialize.
type MaterializeArgs struct {
	// Config is the configuration for the rule's directory.
	Config *config.Config

	// Graph is the frozen build graph.
	Graph *Graph

	// Rule is the generated rule to update, usually its "deps" attribute.
	Rule *rule.Rule

	// Node is Rule's key in Graph, and From its label.
	Node NodeKey
	From label.Label

	// File is the build file containing the rule.
	File *rule.File

	// Cache is an in-memory cache of file metadata.
	Cache *walk.Cache
}

// LinkPolicy customizes how Gazelle links an import required by one of the
// extension's rules. Without one, Gazelle links an import to its single
// provider, reports an error if there are several, and asks
// resolve.Finder implementations if there are none.
type LinkPolicy interface {
	Link(context.Context, LinkArgs) (LinkResult, error)
}

// LinkArgs are the arguments to LinkPolicy.Link.
type LinkArgs struct {
	// Config is the configuration for the requiring rule's directory.
	Config *config.Config

	// From is the requiring rule's label, FromNode its key, and RuleKind
	// its kind.
	From     label.Label
	FromNode NodeKey
	RuleKind string

	// Attr and Select are where the resulting dependency will be recorded.
	Attr, Select string

	// Import is the required import. Lang is the requiring rule's language.
	Import resolve.ImportSpec
	Lang   string

	// Candidates are the targets providing Import, after cross-language
	// filtering and embed suppression, with self imports removed. Sorted by
	// key. Policies aren't called for imports with a "# gazelle:resolve"
	// override.
	Candidates []Node

	// SelfImport is true if a provider was removed from Candidates because
	// depending on it would be a self import. The default policy drops the
	// dependency in that case.
	SelfImport bool

	// Graph is a read-only view of the facts collected so far, for queries.
	// Dependencies are not linked yet.
	Graph *Graph

	// Find calls the resolve.Finder implementations enabled for the
	// directory (honoring -lang), the fallback Gazelle uses without a
	// policy. It's a function rather than a list of finders so that policies
	// get exactly the filtering and arguments the default policy uses.
	Find func(context.Context) ([]resolve.FindResult, error)
}

// LinkResult is returned by LinkPolicy.Link.
type LinkResult struct {
	// To lists the nodes the import links to, usually zero or one. Build
	// keys for labels with TargetKey. An empty To with a nil Err drops the
	// dependency, for example for standard library imports. Self imports in
	// To are dropped.
	To []NodeKey

	// Err records the import as unresolved. If it doesn't wrap
	// ErrAmbiguousImport, it's wrapped with ErrUnresolvedImport. To abort
	// Gazelle instead, return an error from Link.
	Err error
}
