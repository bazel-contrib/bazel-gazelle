/* Copyright 2025 The Bazel Authors. All rights reserved.

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

// Package compat provides interfaces and adapters, supporting compatibility
// between Gazelle v1 and v2.
package compat

import (
	"context"
	"flag"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	configv1 "github.com/bazelbuild/bazel-gazelle/config"
	languagev1 "github.com/bazelbuild/bazel-gazelle/language"
	resolvev1 "github.com/bazelbuild/bazel-gazelle/resolve"
)

// FlagConfigurer allows an extension to define and validate command-line flags.
//
// The v2 Configurer interface does not have these methods, and gazelle v2
// ignores any provided implementations. However, gazelle v1 needs to work
// with the same code, and v1 extensions can provide flags, so we still
// support this interface for compatibility. This interface's methods are
// identical to the corresponding methods in the v1 interface.
type FlagConfigurer interface {
	// RegisterFlags registers command-line flags used by the extension. This
	// method is called once with the root configuration when Gazelle
	// starts. RegisterFlags may set an initial values in Config.Exts. When flags
	// are set, they should modify these values.
	RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config)

	// CheckFlags validates the configuration after command line flags are parsed.
	// This is called once with the root configuration when Gazelle starts.
	// CheckFlags may set default values in flags or make implied changes.
	CheckFlags(fs *flag.FlagSet, c *config.Config) error
}

var _ FlagConfigurer = (configv1.Configurer)(nil)

// ConfigurerV2 returns v and true if v satisfies the v2 Configurer interface,
// or an adapter and true if v satisifes the v1 Configurer interface.
// ConfigurerV2 returns nil and false if v does not satisfy either interface.
func ConfigurerV2(v any) (config.Configurer, bool) {
	switch v := v.(type) {
	case config.Configurer:
		return v, true
	case configv1.Configurer:
		return configurerAdapter{v1: v}, true
	default:
		return nil, false
	}
}

func MustConfigurerV2(v any) config.Configurer {
	return Must(ConfigurerV2(v))
}

type configurerAdapter struct {
	v1 configv1.Configurer
}

var _ config.Configurer = configurerAdapter{}
var _ FlagConfigurer = configurerAdapter{}

func (c configurerAdapter) RegisterFlags(fs *flag.FlagSet, cmd string, cfg *config.Config) {
	c.v1.RegisterFlags(fs, cmd, cfg)
}

func (c configurerAdapter) CheckFlags(fs *flag.FlagSet, cfg *config.Config) error {
	return c.v1.CheckFlags(fs, cfg)
}

func (c configurerAdapter) KnownDirectives() []string {
	return c.v1.KnownDirectives()
}

func (c configurerAdapter) Configure(ctx context.Context, args config.ConfigureArgs) error {
	c.v1.Configure(args.Config, args.Rel, args.File)
	return nil
}

// IndexerV2 wraps a v2 Resolver, adapting it to the v2 Indexer interface.
func IndexerV2(v1 resolvev1.Resolver) resolve.Indexer {
	return indexerAdapter{v1: v1}
}

type indexerAdapter struct {
	v1 resolvev1.Resolver
}

var _ resolve.Indexer = indexerAdapter{}

// TODO(v2): remove
func (i indexerAdapter) Name() string {
	return i.v1.Name()
}

func (i indexerAdapter) Imports(ctx context.Context, args resolve.ImportsArgs) (resolve.ImportsResult, error) {
	imps := i.v1.Imports(args.Config, args.Rule, args.File)
	embeds := i.v1.Embeds(args.Rule, args.From)
	return resolve.ImportsResult{
		Imports:       imps,
		Embeds:        embeds,
		NotImportable: imps == nil,
	}, nil
}

// ResolverV2 wraps a v2 Resolver, adapting it to the v2 Resolver interface.
func ResolverV2(v1 resolvev1.Resolver) resolve.Resolver {
	return resolverAdapter{v1: v1}
}

type resolverAdapter struct {
	v1 resolvev1.Resolver
}

func (r resolverAdapter) Resolve(ctx context.Context, args resolve.ResolveArgs) error {
	r.v1.Resolve(args.Config, resolvev1.WrapRuleIndexV2(args.Index), args.RemoteCache, args.Rule, args.Imports, args.From)
	return nil
}

// FinderV2 wraps a v2 CrossResolver, adapting it to the v2 Finder interface.
func FinderV2(v1 resolvev1.CrossResolver) resolve.Finder {
	return finderAdapter{v1: v1}
}

type finderAdapter struct {
	v1 resolvev1.CrossResolver
}

func (a finderAdapter) Find(ctx context.Context, args resolve.FindArgs) ([]resolve.FindResult, error) {
	return a.v1.CrossResolve(args.Config, resolvev1.WrapRuleIndexV2(args.Index), args.Import, args.Lang), nil
}

type generatorAdapter struct {
	v1 languagev1.Language
}

func (g generatorAdapter) Kinds() map[string]rule.KindInfo {
	return g.v1.Kinds()
}

func (g generatorAdapter) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	result := g.v1.GenerateRules(languagev1.GenerateArgs{
		Config:       args.Config,
		Dir:          args.Dir,
		Rel:          args.Rel,
		File:         args.File,
		Subdirs:      args.Subdirs,
		RegularFiles: args.RegularFiles,
		GenFiles:     args.GenFiles,
		OtherEmpty:   args.OtherEmpty,
		OtherGen:     args.OtherGen,
	})
	return language.GenerateResult{
		Gen:         result.Gen,
		Empty:       result.Empty,
		Imports:     result.Imports,
		RelsToIndex: result.RelsToIndex,
	}, nil
}

type ApparentLoader interface {
	ApparentLoads(moduleToApparentName func(string) string) []rule.LoadInfo
}

type apparentLoaderAdapter struct {
	v1 languagev1.Language
}

func (l apparentLoaderAdapter) ApparentLoads(moduleNameToApparentName func(string) string) []rule.LoadInfo {
	if moduleAware, ok := l.v1.(languagev1.ModuleAwareLanguage); ok {
		// Do not let the extension do its own module name mapping. We'll do this in
		// a centralized place for all extensions.
		return moduleAware.ApparentLoads(moduleNameToApparentName)
	} else {
		return l.v1.Loads()
	}
}

type noopLoader struct{}

func (l noopLoader) ApparentLoads(moduleToApparentName func(string) string) []rule.LoadInfo {
	return nil
}

type fixerAdapter struct {
	v1 languagev1.Language
}

func (f fixerAdapter) Fix(ctx context.Context, args language.FixArgs) error {
	f.v1.Fix(args.Config, args.File)
	return nil
}

type lifecycleAdapter struct {
	v1 languagev1.LifecycleManager
}

func (a lifecycleAdapter) OnStart(ctx context.Context) error {
	a.v1.Before(ctx)
	return nil
}

func (a lifecycleAdapter) OnResolve(ctx context.Context) error {
	a.v1.DoneGeneratingRules()
	return nil
}

func (a lifecycleAdapter) OnFinish(ctx context.Context) error {
	a.v1.AfterResolvingDeps(ctx)
	return nil
}

type finishableAdapter struct {
	v1 languagev1.FinishableLanguage
}

func (a finishableAdapter) OnResolve(ctx context.Context) error {
	a.v1.DoneGeneratingRules()
	return nil
}

// CompleteLanguage satisfies all of the v2 extension interfaces. It may be
// constructed from an extension that partially implements the interfaces using
// LanguageWithDefaults or LanguageV2, which add adapters and stubs as needed.
// Clients of the extension mechanism (namely v2/cmd/gazelle/update) mainly use
// this instead of interacting with an extension directly.
type CompleteLanguage struct {
	language.Language
	language.Generator
	ApparentLoader
	language.Fixer
	language.OnStarter
	language.OnResolver
	language.OnFinisher
	config.Configurer
	FlagConfigurer
	resolve.Indexer
	resolve.Resolver
	resolve.Finder
}

func (a CompleteLanguage) Name() string {
	return a.Language.Name()
}

// LanguageWithDefaults accepts an extension implementing some of the v2
// interfaces and returns a CompleteLanguage, adding stubs for any interfaces
// that the extension didn't implement.
func LanguageWithDefaults(v language.Language) CompleteLanguage {
	adapter := CompleteLanguage{Language: v}
	if gen, ok := v.(language.Generator); ok {
		adapter.Generator = gen
	} else {
		adapter.Generator = noopGenerator{}
	}
	adapter.ApparentLoader = noopLoader{}
	if loader, ok := v.(ApparentLoader); ok {
		adapter.ApparentLoader = loader
	}
	if fix, ok := v.(language.Fixer); ok {
		adapter.Fixer = fix
	} else {
		adapter.Fixer = noopFixer{}
	}
	if start, ok := v.(language.OnStarter); ok {
		adapter.OnStarter = start
	} else {
		adapter.OnStarter = noopOnStarter{}
	}
	if resolve, ok := v.(language.OnResolver); ok {
		adapter.OnResolver = resolve
	} else {
		adapter.OnResolver = noopOnResolver{}
	}
	if finish, ok := v.(language.OnFinisher); ok {
		adapter.OnFinisher = finish
	} else {
		adapter.OnFinisher = noopOnFinisher{}
	}
	if cfg, ok := v.(config.Configurer); ok {
		adapter.Configurer = cfg
	} else if cfg, ok := v.(configv1.Configurer); ok {
		// TODO(v2): migrate internal configurers and stop supporting this.
		adapter.Configurer = configurerAdapter{v1: cfg}
	} else {
		adapter.Configurer = noopConfigurer{}
	}
	// TODO(v2): ignore a v2 implementation that sets this.
	if flag, ok := v.(FlagConfigurer); ok {
		adapter.FlagConfigurer = flag
	} else {
		adapter.FlagConfigurer = noopFlagConfigurer{}
	}
	if idx, ok := v.(resolve.Indexer); ok {
		adapter.Indexer = idx
	} else {
		adapter.Indexer = noopIndexer{Language: v}
	}
	if res, ok := v.(resolve.Resolver); ok {
		adapter.Resolver = res
	} else {
		adapter.Resolver = noopResolver{}
	}
	if find, ok := v.(resolve.Finder); ok {
		adapter.Finder = find
	} else {
		adapter.Finder = noopFinder{}
	}
	return adapter
}

// LanguageV2 accepts an extension implementing the v1 interface and returns a
// CompleteLanguage, wrapping the old extension with adapters and adding stubs
// for interfaces that weren't implemented.
func LanguageV2(v languagev1.Language) CompleteLanguage {
	adapter := CompleteLanguage{
		Language:       v,
		Generator:      generatorAdapter{v1: v},
		ApparentLoader: apparentLoaderAdapter{v1: v},
		Fixer:          fixerAdapter{v1: v},
		Configurer:     configurerAdapter{v1: v},
		FlagConfigurer: configurerAdapter{v1: v},
		Indexer:        indexerAdapter{v1: v},
		Resolver:       resolverAdapter{v1: v},
	}
	if lf, ok := v.(languagev1.LifecycleManager); ok {
		adapter.OnStarter = lifecycleAdapter{v1: lf}
		adapter.OnResolver = lifecycleAdapter{v1: lf}
		adapter.OnFinisher = lifecycleAdapter{v1: lf}
	} else if f, ok := v.(languagev1.FinishableLanguage); ok {
		adapter.OnStarter = noopOnStarter{}
		adapter.OnResolver = finishableAdapter{v1: f}
		adapter.OnFinisher = noopOnFinisher{}
	} else {
		adapter.OnStarter = noopOnStarter{}
		adapter.OnResolver = noopOnResolver{}
		adapter.OnFinisher = noopOnFinisher{}
	}
	if cr, ok := v.(resolvev1.CrossResolver); ok {
		adapter.Finder = finderAdapter{v1: cr}
	} else {
		adapter.Finder = noopFinder{}
	}
	return adapter
}

type noopGenerator struct{}

func (noopGenerator) Kinds() map[string]rule.KindInfo {
	return nil
}

func (noopGenerator) Generate(_ context.Context, _ language.GenerateArgs) (language.GenerateResult, error) {
	return language.GenerateResult{}, nil
}

type noopFixer struct{}

func (noopFixer) Fix(_ context.Context, _ language.FixArgs) error {
	return nil
}

type noopOnStarter struct{}

func (noopOnStarter) OnStart(_ context.Context) error {
	return nil
}

type noopOnResolver struct{}

func (noopOnResolver) OnResolve(_ context.Context) error {
	return nil
}

type noopOnFinisher struct{}

func (noopOnFinisher) OnFinish(_ context.Context) error {
	return nil
}

type noopConfigurer struct{}

func (noopConfigurer) KnownDirectives() []string {
	return nil
}

func (noopConfigurer) Configure(_ context.Context, _ config.ConfigureArgs) error {
	return nil
}

type noopFlagConfigurer struct{}

func (noopFlagConfigurer) RegisterFlags(fs *flag.FlagSet, cmd string, cfg *config.Config) {
}

func (noopFlagConfigurer) CheckFlags(fs *flag.FlagSet, cfg *config.Config) error {
	return nil
}

type noopIndexer struct {
	language.Language
}

func (noopIndexer) Imports(_ context.Context, _ resolve.ImportsArgs) (resolve.ImportsResult, error) {
	return resolve.ImportsResult{}, nil
}

type noopResolver struct{}

func (noopResolver) Resolve(_ context.Context, _ resolve.ResolveArgs) error {
	return nil
}

type noopFinder struct{}

func (noopFinder) Find(_ context.Context, _ resolve.FindArgs) ([]resolve.FindResult, error) {
	return nil, nil
}

func Must[T any](v T, ok bool) T {
	if !ok {
		panic("Must failed")
	}
	return v
}
