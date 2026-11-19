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

// Package ingest turns rules in build files into build graph facts.
//
// It is used for both construction paths: rules Gazelle just generated and
// merged into a file, and rules read from existing build files that Gazelle
// indexes without updating. In both cases the input is a *rule.File after
// merging, so the graph sees exactly the rule identities that will be written.
package ingest

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	graph "github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/model"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

// Attribute classes. Attributes not listed here and not in a kind's
// ResolveAttrs are not ingested; extensions can contribute other edges
// through graph.Facts.
var (
	// fileAttrs list files. Same-package values that don't name a rule or a
	// rule output are source files, recorded with Contains edges.
	fileAttrs = map[string]bool{"srcs": true, "hdrs": true, "textual_hdrs": true, "embedsrcs": true}

	// depAttrs list dependencies.
	depAttrs = map[string]bool{
		"deps": true, "runtime_deps": true, "implementation_deps": true,
		"data": true, "tools": true, "plugins": true, "exports": true,
		"embed": true, "actual": true,
	}

	// outAttrs list outputs.
	outAttrs = map[string]bool{"out": true, "outs": true}
)

// Rule describes one rule to ingest.
type Rule struct {
	// Rule is the rule as it appears in the (merged) build file.
	Rule *rule.Rule

	// Name overrides Rule.Name() as the rule's name in the graph. It's used
	// when Rule is a generated rule that was merged into an existing rule
	// with a different name.
	Name string

	// Info is the rule's kind information. Its ResolveAttrs are treated as
	// dependency attributes.
	Info rule.KindInfo

	// Lang is the extension owning the rule, or "" if unknown.
	Lang string

	// Origin is the origin of facts about the rule.
	Origin graph.Origin

	// SkipAttrs are attributes Gazelle will derive again in this run. Only
	// values marked "# keep" are ingested from them, since the others may be
	// stale. The derived values are added later.
	SkipAttrs map[string]bool
}

// Package describes a package to ingest.
type Package struct {
	// RepoName is the main repository's name (config.Config.RepoName). Labels
	// with this repo name are normalized to the main repository.
	RepoName string

	// Pkg is the package path, relative to the repository root.
	Pkg string

	Rules []Rule
}

// Ingest adds nodes and edges for every rule in p.
func Ingest(b *graph.Builder, p Package) error {
	ctx := newPkgContext(p)
	var errs []error
	for _, r := range p.Rules {
		if r.name() == "" {
			continue
		}
		if err := b.AddNode(graph.Node{
			Key:      ctx.ruleKey(r.name()),
			RuleKind: r.Rule.Kind(),
			Lang:     r.Lang,
			Origin:   origin(r, r.Rule.ShouldKeep()),
		}); err != nil {
			errs = append(errs, err)
			continue
		}
		for _, attr := range r.Rule.AttrKeys() {
			if !isLabelAttr(attr, r.Info) {
				continue
			}
			errs = append(errs, ctx.ingestAttr(b, r, attr, r.SkipAttrs[attr])...)
		}
	}
	return errors.Join(errs...)
}

// IngestAttrs adds edges for the given attributes of the observed rules,
// regardless of SkipAttrs. It's used after legacy dependency resolution to
// record the dependencies a Resolver wrote (the "observed" graph). p
// describes the whole package (it's used to tell rules from files); observed
// are the resolved rules, usually generated rules with Name set to the name
// of the rule they were merged into.
func IngestAttrs(b *graph.Builder, p Package, observed []Rule, attrs func(Rule) []string) error {
	p.Rules = append(slices.Clip(p.Rules), observed...)
	ctx := newPkgContext(p)
	var errs []error
	for _, r := range observed {
		if r.name() == "" {
			continue
		}
		for _, attr := range attrs(r) {
			if r.Rule.Attr(attr) != nil {
				errs = append(errs, ctx.ingestAttr(b, r, attr, false)...)
			}
		}
	}
	return errors.Join(errs...)
}

// RuleKey returns the key for a rule named name in package pkg of the main
// repository.
func RuleKey(pkg, name string) graph.NodeKey {
	return graph.TargetKey(label.New("", pkg, name))
}

// LabelKey returns the target key for l, written in package pkg of the main
// repository named repoName. Relative labels are made absolute, and labels
// naming the main repository are normalized.
func LabelKey(l label.Label, repoName, pkg string) graph.NodeKey {
	return graph.TargetKey(normalize(l, repoName, pkg))
}

func normalize(l label.Label, repoName, pkg string) label.Label {
	l = l.Abs("", pkg)
	if l.Repo == "@" || (l.Repo != "" && l.Repo == repoName && !l.Canonical) {
		l.Repo, l.Canonical = "", false
	}
	return l
}

type pkgContext struct {
	repoName, pkg string
	rules         map[string]bool
	outputs       map[string]string // output name -> rule name
}

func newPkgContext(p Package) *pkgContext {
	ctx := &pkgContext{repoName: p.RepoName, pkg: p.Pkg, rules: map[string]bool{}, outputs: map[string]string{}}
	for _, r := range p.Rules {
		ctx.rules[r.name()] = true
	}
	for _, r := range p.Rules {
		for attr := range outAttrs {
			for _, v := range values(r.Rule.Attr(attr), false) {
				if l, err := label.Parse(v.s); err == nil && l.Relative {
					ctx.outputs[l.Name] = r.name()
				}
			}
		}
	}
	return ctx
}

func (r Rule) name() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Rule.Name()
}

func (c *pkgContext) ruleKey(name string) graph.NodeKey { return RuleKey(c.pkg, name) }

func isLabelAttr(attr string, info rule.KindInfo) bool {
	return fileAttrs[attr] || depAttrs[attr] || outAttrs[attr] || info.ResolveAttrs[attr]
}

func origin(r Rule, keep bool) graph.Origin {
	if keep {
		return r.Origin | graph.OriginKeep
	}
	return r.Origin
}

func (c *pkgContext) ingestAttr(b *graph.Builder, r Rule, attr string, keptOnly bool) []error {
	from := c.ruleKey(r.name())
	attrKeep := r.Rule.ShouldKeep() || hasKeep(r.Rule.AttrComments(attr))
	var errs []error
	add := func(to graph.NodeKey, e graph.Edge, keep bool) {
		if err := b.AddEdge(from, to, e, origin(r, keep)); err != nil {
			errs = append(errs, fmt.Errorf("%s: attribute %q: %w", from, attr, err))
		}
	}
	for _, v := range values(r.Rule.Attr(attr), attrKeep) {
		if keptOnly && !v.keep {
			continue
		}
		l, err := label.Parse(v.s)
		if err != nil {
			continue // Not a label; for example, a make variable.
		}
		l = normalize(l, c.repoName, c.pkg)

		if outAttrs[attr] {
			if l.Repo == "" && l.Pkg == c.pkg {
				add(graph.GeneratedFileKey(l), graph.Generates{}, v.keep)
			}
			continue
		}

		var to graph.NodeKey
		isFile := false
		switch {
		case l.Repo != "" || l.Pkg != c.pkg:
			// Another package. Without loading it we can't tell rules from
			// files; treat it as a target.
			to = graph.TargetKey(l)
		case c.rules[l.Name]:
			to = graph.TargetKey(l)
		case c.outputs[l.Name] != "":
			to = graph.GeneratedFileKey(l)
		default:
			to, isFile = graph.SourceFileKey(l), true
		}

		switch {
		case attr == "actual":
			add(to, graph.AliasOf{}, v.keep)
		case attr == "exports":
			add(to, graph.Exports{}, v.keep)
		case isFile && fileAttrs[attr]:
			add(to, graph.Contains{Attr: attr}, v.keep)
		default:
			// Embeds edges come from Indexer results, not the "embed"
			// attribute, whose meaning is language-specific.
			add(to, graph.DependsOn{Attr: attr, Select: v.sel}, v.keep)
		}
	}
	return errs
}

type value struct {
	s    string
	sel  string
	keep bool
}

// values extracts string values from an attribute expression: a string, a
// list of strings, select() calls, and concatenations of these. Each value
// records the select() key it appears under and whether it's kept.
func values(e bzl.Expr, keep bool) []value {
	var vs []value
	var walk func(e bzl.Expr, sel string, keep bool)
	walk = func(e bzl.Expr, sel string, keep bool) {
		if e == nil {
			return
		}
		keep = keep || rule.ShouldKeep(e)
		switch e := e.(type) {
		case *bzl.StringExpr:
			vs = append(vs, value{s: e.Value, sel: sel, keep: keep})
		case *bzl.ListExpr:
			for _, x := range e.List {
				walk(x, sel, keep)
			}
		case *bzl.BinaryExpr:
			if e.Op == "+" {
				walk(e.X, sel, keep)
				walk(e.Y, sel, keep)
			}
		case *bzl.CallExpr:
			if x, ok := e.X.(*bzl.Ident); !ok || x.Name != "select" || len(e.List) != 1 {
				return
			}
			dict, ok := e.List[0].(*bzl.DictExpr)
			if !ok {
				return
			}
			for _, kv := range dict.List {
				key, ok := kv.Key.(*bzl.StringExpr)
				if !ok {
					continue
				}
				walk(kv.Value, key.Value, keep || rule.ShouldKeep(kv))
			}
		}
	}
	walk(e, "", keep)
	slices.SortStableFunc(vs, func(a, b value) int { return strings.Compare(a.s, b.s) })
	return vs
}

func hasKeep(c *bzl.Comments) bool {
	if c == nil {
		return false
	}
	for _, x := range append(slices.Clone(c.Before), c.Suffix...) {
		text := strings.TrimSpace(strings.TrimPrefix(x.Token, "#"))
		if text == "keep" || strings.HasPrefix(text, "keep: ") {
			return true
		}
	}
	return false
}
