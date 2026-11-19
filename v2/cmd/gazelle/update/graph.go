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

package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/compat"
	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	gzerrors "github.com/bazel-contrib/bazel-gazelle/v2/errors"
	"github.com/bazel-contrib/bazel-gazelle/v2/graph"
	"github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/ingest"
	"github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/link"
	"github.com/bazel-contrib/bazel-gazelle/v2/internal/graph/model"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/merger"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/bazel-contrib/bazel-gazelle/v2/walk"
)

// cycleMode is the value of -graph_cycles and "# gazelle:graph_cycles".
type cycleMode string

const (
	cyclesError   cycleMode = "error"
	cyclesWarning cycleMode = "warning"
	cyclesOff     cycleMode = "off"
)

func parseCycleMode(s string) (cycleMode, error) {
	switch m := cycleMode(s); m {
	case cyclesError, cyclesWarning, cyclesOff:
		return m, nil
	default:
		return "", fmt.Errorf("invalid graph_cycles value %q: want error, warning, or off", s)
	}
}

// graphRun holds build graph state for one run of Gazelle with
// -experimental_build_graph.
//
// The phases are:
//
//   - collect (during the walk): rules in each visited build file become
//     nodes and edges; extensions add facts with graph.Contributor.
//   - link (after the walk): Requires edges become DependsOn edges.
//   - resolve: rules owned by extensions without a Materializer are resolved
//     with their legacy Resolver, and the dependencies they wrote are added
//     to the graph ("observed" edges), so cycle detection covers them too.
//   - freeze and validate: cycles and unresolved imports are reported.
//   - materialize: Materializers set attributes from the frozen graph.
type graphRun struct {
	b        *model.Builder
	g        *graph.Graph
	repoName string
	mrslv    *metaResolver
	cache    *walk.Cache

	// ifaces holds each extension's graph interfaces, by extension name.
	ifaces  map[string]graphInterfaces
	accepts map[string][]string
	configs map[string]*config.Config

	// imports caches Indexer.Imports results per rule; see indexer.
	imports map[*rule.Rule]importsResult
}

// graphInterfaces holds the optional graph interfaces an extension
// implements; nil fields are not implemented. v1 extensions implement none.
type graphInterfaces struct {
	schema       graph.SchemaDeclarer
	contributor  graph.Contributor
	materializer graph.Materializer
	linkPolicy   graph.LinkPolicy
}

func lookupGraphInterfaces(lang compat.CompleteLanguage) graphInterfaces {
	var gi graphInterfaces
	gi.schema, _ = lang.Language.(graph.SchemaDeclarer)
	gi.contributor, _ = lang.Language.(graph.Contributor)
	gi.materializer, _ = lang.Language.(graph.Materializer)
	gi.linkPolicy, _ = lang.Language.(graph.LinkPolicy)
	return gi
}

// graphNamePrivateAttr is a private attribute set on generated rules with
// the name of the rule they were merged into: their node name in the graph.
// It's unset if the rule wasn't merged or was merged into a kept rule.
const graphNamePrivateAttr = "_gazelle_graph_name"

func graphName(r *rule.Rule) string {
	name, _ := r.PrivateAttr(graphNamePrivateAttr).(string)
	return name
}

// newGraphRun prepares graph state. A returned error should be handled with
// errorHandler; if it's critical, gr is nil.
func newGraphRun(ctx context.Context, c *config.Config, languages []compat.CompleteLanguage, mrslv *metaResolver, cache *walk.Cache) (gr *graphRun, err error) {
	var schemas []graph.EdgeSchema
	ifaces := make(map[string]graphInterfaces)
	accepts := make(map[string][]string)
	var errs []error
	for _, lang := range languages {
		gi := lookupGraphInterfaces(lang)
		if gi.contributor != nil && gi.materializer == nil {
			// Rules of this extension are resolved by its Resolver and
			// observed; linking their facts too would duplicate edges and
			// unresolved-import reports.
			errs = append(errs, gzerrors.SeverityErrorf(gzerrors.Warning, "%s: implements graph.Contributor without graph.Materializer; its graph facts are ignored", lang.Name()))
			gi.contributor = nil
		}
		ifaces[lang.Name()] = gi
		if gi.schema == nil {
			continue
		}
		res, err := gi.schema.DeclareGraphSchema(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: graph schema: %w", lang.Name(), err))
			continue
		}
		for _, s := range res.Edges {
			if !strings.HasPrefix(string(s.Kind), lang.Name()+"/") {
				errs = append(errs, fmt.Errorf("%s: graph schema: edge kind %q must start with %q", lang.Name(), s.Kind, lang.Name()+"/"))
				continue
			}
			schemas = append(schemas, s)
		}
		accepts[lang.Name()] = res.AcceptProvidersFrom
	}
	// Schemas were validated above, so this only fails on programming errors.
	b, err := model.NewBuilder(model.BuilderOptions{Schemas: schemas})
	if err != nil {
		return nil, gzerrors.WithSeverity(gzerrors.Critical, fmt.Errorf("build graph: %w", err))
	}
	return &graphRun{
		b:        b,
		repoName: c.RepoName,
		mrslv:    mrslv,
		cache:    cache,
		ifaces:   ifaces,
		accepts:  accepts,
		configs:  make(map[string]*config.Config),
		imports:  make(map[*rule.Rule]importsResult),
	}, errors.Join(errs...)
}

// ruleLang returns the name of the extension that indexes r, which is the
// language RuleIndex associates with r. Generated rules carry their
// generating extension in a private attribute; metaResolver checks it first.
func (gr *graphRun) ruleLang(r *rule.Rule, pkg string) string {
	if idx := gr.mrslv.Indexer(r, pkg); idx != nil {
		return idx.Name()
	}
	if lang, ok := r.PrivateAttr(langPrivateAttr).(language.Language); ok {
		return lang.Name()
	}
	return ""
}

// indexer returns the Indexer for r, like metaResolver.Indexer, wrapped so
// that Imports runs once per rule even though both RuleIndex and the graph
// ask for it.
func (gr *graphRun) indexer(r *rule.Rule, pkg string) resolve.Indexer {
	idx := gr.mrslv.Indexer(r, pkg)
	if idx == nil {
		return nil
	}
	return memoIndexer{Indexer: idx, gr: gr}
}

type importsResult struct {
	res resolve.ImportsResult
	err error
}

type memoIndexer struct {
	resolve.Indexer
	gr *graphRun
}

func (m memoIndexer) Imports(ctx context.Context, args resolve.ImportsArgs) (resolve.ImportsResult, error) {
	if r, ok := m.gr.imports[args.Rule]; ok {
		return r.res, r.err
	}
	res, err := m.Indexer.Imports(ctx, args)
	m.gr.imports[args.Rule] = importsResult{res, err}
	return res, err
}

// addProvides adds Provides and Embeds edges for r from its Indexer, like
// RuleIndex.AddRule.
func (gr *graphRun) addProvides(ctx context.Context, c *config.Config, f *rule.File, r *rule.Rule, name string, o graph.Origin) error {
	idx := gr.indexer(r, f.Pkg)
	if idx == nil || !passesLanguageFilter(c.Langs, idx.Name()) {
		return nil
	}
	res, err := idx.Imports(ctx, resolve.ImportsArgs{Config: c, Rule: r, File: f, Cache: gr.cache})
	if err != nil {
		return fmt.Errorf("language %s: %w", idx.Name(), err)
	}
	from := ingest.RuleKey(f.Pkg, name)
	var errs []error
	for _, imp := range res.Imports {
		errs = append(errs, gr.b.AddEdge(from, graph.ImportKey(imp), model.Provides{}, o))
	}
	for _, e := range res.Embeds {
		errs = append(errs, gr.b.AddEdge(from, ingest.LabelKey(e, gr.repoName, f.Pkg), model.Embeds{}, o))
	}
	return errors.Join(errs...)
}

func passesLanguageFilter(langs []string, name string) bool {
	if len(langs) == 0 {
		return true
	}
	for _, l := range langs {
		if l == name {
			return true
		}
	}
	return false
}

// indexPackage ingests a build file in a directory Gazelle indexes but
// doesn't update. All of its attributes are facts.
func (gr *graphRun) indexPackage(ctx context.Context, c *config.Config, f *rule.File, kinds map[string]rule.KindInfo) error {
	gr.configs[f.Pkg] = c
	p := ingest.Package{RepoName: gr.repoName, Pkg: f.Pkg}
	for _, r := range f.Rules {
		p.Rules = append(p.Rules, ingest.Rule{Rule: r, Info: kinds[r.Kind()], Lang: gr.ruleLang(r, f.Pkg), Origin: graph.OriginIndexed})
	}
	errs := []error{ingest.Ingest(gr.b, p)}
	for _, r := range f.Rules {
		if r.Name() != "" {
			errs = append(errs, gr.addProvides(ctx, c, f, r, r.Name(), graph.OriginIndexed))
		}
	}
	return errors.Join(errs...)
}

// collectArgs describes a directory Gazelle updated, after generated rules
// were merged into its build file.
type collectArgs struct {
	c            *config.Config
	f            *rule.File
	gen          []*rule.Rule
	getKindInfo  func(*rule.Rule) rule.KindInfo
	aliasedKinds map[string]string
	languages    []compat.CompleteLanguage // languages enabled for the directory
}

// collect ingests an updated directory. It records the graph name of each
// generated rule in a private attribute (see graphName).
func (gr *graphRun) collect(ctx context.Context, a collectArgs) error {
	f := a.f
	gr.configs[f.Pkg] = a.c

	names := make([]string, len(a.gen))
	// final maps each rule in the file that a generated rule was merged into
	// to that generated rule. The generated rule's kind info is used: the
	// file's rule may have a wrapper kind (alias_kind) unknown to kinds.
	final := make(map[*rule.Rule]*rule.Rule)
	for i, g := range a.gen {
		// Match errors are skipped: MergeFile skipped the same rules for
		// the same errors (without reporting them; see merger.MergeFile).
		m, err := merger.Match(f.Rules, g, a.getKindInfo(g), a.aliasedKinds)
		if err != nil || m == nil || m.ShouldKeep() {
			continue
		}
		names[i] = m.Name()
		g.SetPrivateAttr(graphNamePrivateAttr, m.Name())
		final[m] = g
	}

	p := ingest.Package{RepoName: gr.repoName, Pkg: f.Pkg}
	for _, r := range f.Rules {
		ir := ingest.Rule{Rule: r, Info: a.getKindInfo(r), Lang: gr.ruleLang(r, f.Pkg), Origin: graph.OriginExisting}
		if g := final[r]; g != nil {
			ir.Info = a.getKindInfo(g)
			// Dependency attributes of generated rules are derived again by
			// linking or resolution; only kept values are facts.
			ir.Origin = graph.OriginGenerated
			ir.SkipAttrs = ir.Info.ResolveAttrs
		}
		p.Rules = append(p.Rules, ir)
	}
	errs := []error{ingest.Ingest(gr.b, p)}
	if a.c.IndexLibraries {
		for _, ir := range p.Rules {
			if ir.Rule.Name() != "" {
				errs = append(errs, gr.addProvides(ctx, a.c, f, ir.Rule, ir.Rule.Name(), ir.Origin))
			}
		}
	}

	for _, lang := range a.languages {
		contributor := gr.ifaces[lang.Name()].contributor
		if contributor == nil {
			continue
		}
		args := graph.ContributeArgs{
			Config: a.c,
			Rel:    f.Pkg,
			File:   f,
			Facts:  gr.b.PackageFacts(lang.Name(), f.Pkg, graph.OriginGenerated),
			Cache:  gr.cache,
		}
		for i, r := range a.gen {
			if owner, ok := r.PrivateAttr(langPrivateAttr).(compat.CompleteLanguage); !ok || owner.Name() != lang.Name() {
				continue
			}
			args.Gen = append(args.Gen, r)
			args.Imports = append(args.Imports, r.PrivateAttr(importsPrivateAttr))
			var key graph.NodeKey
			if names[i] != "" {
				key = ingest.RuleKey(f.Pkg, names[i])
			}
			args.Nodes = append(args.Nodes, key)
		}
		if err := contributor.ContributeGraph(ctx, args); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", lang.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// link runs the link step.
func (gr *graphRun) link(ctx context.Context, finders []resolve.Finder, ix *resolve.RuleIndex) error {
	return link.Link(ctx, gr.b, link.Options{
		RepoName: gr.repoName,
		Config:   func(pkg string) *config.Config { return gr.configs[pkg] },
		Policy:   func(lang string) graph.LinkPolicy { return gr.ifaces[lang].linkPolicy },
		Accepts:  func(lang string) []string { return gr.accepts[lang] },
		Finders:  finders,
		Index:    ix,
		Cache:    gr.cache,
	})
}

// observe records the dependencies legacy Resolvers wrote into the observed
// generated rules of file f. Each observed rule has Name set to its graph
// name.
func (gr *graphRun) observe(f *rule.File, observed []ingest.Rule) error {
	if len(observed) == 0 {
		return nil
	}
	p := ingest.Package{RepoName: gr.repoName, Pkg: f.Pkg}
	for _, fr := range f.Rules {
		p.Rules = append(p.Rules, ingest.Rule{Rule: fr})
	}
	return ingest.IngestAttrs(gr.b, p, observed, func(ir ingest.Rule) []string {
		var attrs []string
		for a := range ir.Info.ResolveAttrs {
			attrs = append(attrs, a)
		}
		return attrs
	})
}

// freeze freezes the graph and reports cycles and unresolved imports. The
// returned error is critical: the graph couldn't be built.
func (gr *graphRun) freeze(mode cycleMode, eh *errorHandler) error {
	g, err := gr.b.Freeze()
	if err != nil {
		return gzerrors.WithSeverity(gzerrors.Critical, fmt.Errorf("building graph: %w", err))
	}
	gr.g = g
	for _, w := range gr.b.Warnings() {
		eh.handle(nil, gzerrors.SeverityErrorf(gzerrors.Warning, "build graph: %s", w))
	}
	for _, k := range g.UnresolvedAll() {
		for _, u := range g.Unresolved(k) {
			info, _ := graph.ParseKey(k)
			// Legacy resolvers report unresolved imports without failing
			// strict runs; do the same.
			eh.handle(nil, gzerrors.WithSeverity(gzerrors.Warning, fmt.Errorf("%s: %w", info.Label, u.Err)))
		}
	}
	if mode == cyclesOff {
		return nil
	}
	sev := gzerrors.Warning
	if mode == cyclesError {
		sev = gzerrors.Error
	}
	for _, c := range g.Cycles() {
		eh.handle(nil, gzerrors.WithSeverity(sev, &graph.CycleError{Cycle: c}))
	}
	return nil
}

// dump writes the frozen graph to the requested debug files.
func (gr *graphRun) dump(jsonPath, dotPath string) error {
	var errs []error
	for _, d := range []struct {
		path  string
		write func(*bytes.Buffer) error
	}{
		{jsonPath, func(b *bytes.Buffer) error { return gr.g.WriteJSON(b) }},
		{dotPath, func(b *bytes.Buffer) error { return gr.g.WriteDOT(b) }},
	} {
		if d.path == "" {
			continue
		}
		var buf bytes.Buffer
		if err := d.write(&buf); err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, os.WriteFile(d.path, buf.Bytes(), 0o666))
	}
	return errors.Join(errs...)
}

type graphResolveArgs struct {
	visits      []visitRecord
	finders     []resolve.Finder
	ruleIndex   *resolve.RuleIndex
	kinds       map[string]rule.KindInfo
	resolveRule func(visitRecord, *rule.Rule) bool
	uc          *updateConfig
	eh          *errorHandler
}

// resolve runs the phases after the walk: link, legacy resolution with
// observation, freeze and validation, debug dumps, and materialization. It
// returns false if Gazelle should stop.
func (gr *graphRun) resolve(ctx context.Context, a graphResolveArgs) bool {
	eh := a.eh
	if err := gr.link(ctx, a.finders, a.ruleIndex); err != nil && !eh.handle(nil, err) {
		return false
	}

	materializer := func(r *rule.Rule) graph.Materializer {
		owner, _ := r.PrivateAttr(langPrivateAttr).(language.Language)
		if owner == nil {
			return nil
		}
		return gr.ifaces[owner.Name()].materializer
	}
	for _, v := range a.visits {
		getKindInfo := makeGetKindInfo(unionKindInfoMaps(a.kinds, v.mappedKindInfo))
		var observed []ingest.Rule
		for _, r := range v.rules {
			if materializer(r) != nil {
				continue
			}
			if !a.resolveRule(v, r) {
				return false
			}
			if name := graphName(r); name != "" {
				observed = append(observed, ingest.Rule{Rule: r, Name: name, Info: getKindInfo(r), Origin: graph.OriginGenerated})
			}
		}
		if err := gr.observe(v.file, observed); err != nil && !eh.handle(nil, err) {
			return false
		}
	}

	if err := gr.freeze(a.uc.graphCycles, eh); err != nil && !eh.handle(nil, err) {
		return false
	}
	if gr.g == nil {
		return true // freezing failed with a non-critical error; nothing to materialize
	}
	if err := gr.dump(a.uc.graphJSON, a.uc.graphDOT); err != nil && !eh.handle(nil, err) {
		return false
	}

	for _, v := range a.visits {
		for _, r := range v.rules {
			m, name := materializer(r), graphName(r)
			if m == nil || name == "" {
				continue
			}
			err := m.Materialize(ctx, graph.MaterializeArgs{
				Config: v.c,
				Graph:  gr.g,
				Rule:   r,
				Node:   ingest.RuleKey(v.pkgRel, name),
				From:   label.New(gr.repoName, v.pkgRel, name),
				File:   v.file,
				Cache:  gr.cache,
			})
			if err != nil && !eh.handle(r.PrivateAttr(langPrivateAttr), err) {
				return false
			}
		}
	}
	return true
}
