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

// Package link implements the build graph's link step: it replaces each
// Requires edge with DependsOn edges to the targets providing the import.
// This takes over the import-matching half of dependency resolution.
package link

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/graph"
	"github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/ingest"
	model "github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/model"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/walk"
)

// Options configures linking.
type Options struct {
	// RepoName is the main repository's name.
	RepoName string

	// Config returns the configuration for a package. It must not return
	// nil for a package containing a requiring rule.
	Config func(pkg string) *config.Config

	// Policy returns the link policy for an extension, or nil for the
	// default policy.
	Policy func(lang string) graph.LinkPolicy

	// Accepts returns the other extensions whose providers may satisfy
	// imports required by lang's rules, in order of preference.
	Accepts func(lang string) []string

	// Finders are consulted when no target provides an import.
	Finders []resolve.Finder

	// Index is passed to Finder.Find for compatibility. It may be nil.
	Index *resolve.RuleIndex

	Cache *walk.Cache
}

// Link links every pending Requires edge in b. Problems with individual
// imports are recorded in the graph as unresolved imports (see
// graph.Graph.Unresolved), not returned. Link returns an error only for
// failures that leave the builder inconsistent, or when ctx is canceled.
func Link(ctx context.Context, b *model.Builder, opts Options) error {
	snap := b.Snapshot()
	var errs []error
	for _, f := range b.PendingRequires() {
		if err := ctx.Err(); err != nil {
			return err
		}
		to, linkErr := linkOne(ctx, snap, f, opts)
		var err error
		switch {
		case linkErr == nil:
			err = b.Link(f, to, model.OriginGenerated)
		case errors.Is(linkErr, graph.ErrUnresolvedImport) || errors.Is(linkErr, graph.ErrAmbiguousImport):
			err = b.MarkUnresolved(f, linkErr)
		default:
			// A hard failure, for example a canceled context inside a
			// LinkPolicy.
			return fmt.Errorf("linking %s import %q: %w", keyLabel(f.From), f.Import.Imp, linkErr)
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// linkOne decides what one Requires fact links to. A nil, nil result drops
// the dependency. Errors wrapping ErrUnresolvedImport or ErrAmbiguousImport
// are recorded on the graph; others abort linking.
//
// The default policy follows the legacy resolvers (resolve.RuleIndex and the
// Go resolver): overrides first; then providers, reporting ambiguity before
// checking for a self import; then Finders.
func linkOne(ctx context.Context, snap *graph.Graph, f model.RequireFact, opts Options) ([]graph.NodeKey, error) {
	fromNode, _ := snap.Node(f.From)
	from := fromNode.Label
	lang := fromNode.Lang
	c := opts.Config(from.Pkg)
	imp := f.Import

	toKey := func(l label.Label) graph.NodeKey { return ingest.LabelKey(l, opts.RepoName, from.Pkg) }
	isSelf := func(k graph.NodeKey) bool { return snap.IsSelfImport(f.From, k) }

	// 1. "# gazelle:resolve" overrides win.
	if c != nil {
		if l, ok := resolve.FindRuleWithOverride(c, imp, lang); ok {
			if k := toKey(l); !isSelf(k) {
				return []graph.NodeKey{k}, nil
			}
			return nil, nil
		}
	}

	// 2. Providers, preferring the requiring extension's own, then accepted
	// extensions in order.
	cands := candidates(snap, f.Import, lang, opts.Accepts)
	find := func(ctx context.Context) ([]resolve.FindResult, error) {
		return findAll(ctx, c, imp, lang, opts)
	}

	// 3. An extension's policy decides, if it has one.
	if opts.Policy != nil {
		if p := opts.Policy(lang); p != nil {
			var nonSelf []graph.Node
			for _, n := range cands {
				if !isSelf(n.Key) {
					nonSelf = append(nonSelf, n)
				}
			}
			res, err := p.Link(ctx, graph.LinkArgs{
				Config: c, From: from, FromNode: f.From, RuleKind: fromNode.RuleKind,
				Attr: f.Attr, Select: f.Select,
				Import: imp, Lang: lang,
				Candidates: nonSelf, SelfImport: len(nonSelf) < len(cands),
				Graph: snap, Find: find,
			})
			if err != nil {
				return nil, err
			}
			if res.Err != nil {
				if !errors.Is(res.Err, graph.ErrUnresolvedImport) && !errors.Is(res.Err, graph.ErrAmbiguousImport) {
					return nil, fmt.Errorf("%w: %v", graph.ErrUnresolvedImport, res.Err)
				}
				return nil, res.Err
			}
			var to []graph.NodeKey
			for _, k := range res.To {
				if !isSelf(k) {
					to = append(to, k)
				}
			}
			return to, nil
		}
	}

	// 4. Default policy.
	switch {
	case len(cands) > 1:
		return nil, ambiguous(imp, keysOf(cands))
	case len(cands) == 1:
		if isSelf(cands[0].Key) {
			return nil, nil
		}
		return []graph.NodeKey{cands[0].Key}, nil
	}
	results, err := find(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s: %v", graph.ErrUnresolvedImport, imp.Imp, err)
	}
	var to []graph.NodeKey
	for _, r := range results {
		to = append(to, toKey(r.Label))
	}
	slices.Sort(to)
	to = slices.Compact(to)
	switch len(to) {
	case 0:
		return nil, fmt.Errorf("%w: no target provides %q", graph.ErrUnresolvedImport, imp.Imp)
	case 1:
		if isSelf(to[0]) {
			return nil, nil
		}
		return to, nil
	default:
		return nil, ambiguous(imp, to)
	}
}

// candidates returns providers of imp acceptable to lang: providers from
// lang itself if there are any, otherwise providers from the first accepted
// extension that has some.
func candidates(snap *graph.Graph, imp graph.ImportSpec, lang string, accepts func(string) []string) []graph.Node {
	order := []string{lang}
	if accepts != nil {
		order = append(order, accepts(lang)...)
	}
	providers := snap.Providers(imp)
	for _, l := range order {
		var ns []graph.Node
		for _, p := range providers {
			if p.Lang == l {
				ns = append(ns, p)
			}
		}
		if len(ns) > 0 {
			return ns
		}
	}
	return nil
}

func keyLabel(k graph.NodeKey) string {
	if info, err := graph.ParseKey(k); err == nil && info.Class != graph.KindImport {
		return info.Label.String()
	}
	return string(k)
}

func findAll(ctx context.Context, c *config.Config, imp resolve.ImportSpec, lang string, opts Options) ([]resolve.FindResult, error) {
	if c == nil {
		return nil, nil
	}
	var results []resolve.FindResult
	var errs []error
	for _, f := range opts.Finders {
		if len(c.Langs) > 0 && !slices.Contains(c.Langs, f.Name()) {
			continue
		}
		rs, err := f.Find(ctx, resolve.FindArgs{Config: c, Index: opts.Index, Import: imp, Lang: lang, Cache: opts.Cache})
		if err != nil {
			errs = append(errs, fmt.Errorf("finder %s: %w", f.Name(), err))
		}
		results = append(results, rs...)
	}
	return results, errors.Join(errs...)
}

func keysOf(ns []graph.Node) []graph.NodeKey {
	ks := make([]graph.NodeKey, len(ns))
	for i, n := range ns {
		ks[i] = n.Key
	}
	return ks
}

func ambiguous(imp resolve.ImportSpec, ks []graph.NodeKey) error {
	var ls []string
	for _, k := range ks {
		ls = append(ls, keyLabel(k))
	}
	return fmt.Errorf("%w: multiple rules may be imported with %q: %s", graph.ErrAmbiguousImport, imp.Imp, strings.Join(ls, ", "))
}
