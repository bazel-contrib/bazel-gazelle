/* Copyright 2019 The Bazel Authors. All rights reserved.

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
	"context"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

// metaResolver provides a rule.Resolver for any rule.Rule.
//
// TODO(#2421): in v2, only use metaResolver for pre-existing rules.
// For freshly generated rules, call Imports and Resolve on the extension
// that generated the rules.
type metaResolver struct {
	// builtins provides a map of the language kinds to their resolver.
	builtins map[string]indexResolver

	// mappedKinds provides a list of replacements used by File.Pkg.
	mappedKinds map[string][]config.MappedKind

	// aliasedKinds provides a dict of configured wrapper macros for each package
	aliasedKinds map[string]map[string]string
}

type indexResolver interface {
	resolve.Indexer
	resolve.Resolver
}

func newMetaResolver() *metaResolver {
	return &metaResolver{
		builtins:     make(map[string]indexResolver),
		mappedKinds:  make(map[string][]config.MappedKind),
		aliasedKinds: make(map[string]map[string]string),
	}
}

// AddBuiltin registers a builtin kind with its info.
func (mr *metaResolver) AddBuiltin(kindName string, resolver indexResolver) {
	mr.builtins[kindName] = resolver
}

// MappedKind records the fact that the given mapping was applied while
// processing the given package.
func (mr *metaResolver) MappedKind(pkgRel string, kind config.MappedKind) {
	mr.mappedKinds[pkgRel] = append(mr.mappedKinds[pkgRel], kind)
}

// AliasedKinds records the configured wrapper macros for a package
func (mr *metaResolver) AliasedKinds(pkgRel string, aliasedKinds map[string]string) {
	// Note: it is somewhat of a hack to store the aliased kinds in the metaResolver
	// by each package. A more appropriate place for this would be to keep it in the
	// config.Config struct. However, the config.Config struct is not available at
	// all of the call sites where the Resolve method is called.
	//
	// For example, when the RuleIndex is finalizing and collecting information about
	// embedded targets, it does this once across the entire index.
	mr.aliasedKinds[pkgRel] = aliasedKinds
}

// Indexer returns an indexer for the given rule and package. Empty string
// may be passed for pkgRel, which results in consulting the builtin kinds only.
func (mr *metaResolver) Indexer(r *rule.Rule, pkgRel string) resolve.Indexer {
	return mr.find(r, pkgRel)
}

// Resolver returns a resolver for the given rule and package. Empty string
// may be passed for pkgRel, which results in consulting the builtin kinds only.
func (mr *metaResolver) Resolver(r *rule.Rule, pkgRel string) resolve.Resolver {
	return mr.find(r, pkgRel)
}

func (mr *metaResolver) find(r *rule.Rule, pkgRel string) indexResolver {
	ruleKind := r.Kind()

	if wrappedKind, ok := mr.aliasedKinds[pkgRel][ruleKind]; ok {
		ruleKind = wrappedKind
	}

	// Once we have checked alias kinds, still look through our mapped kinds so that we can handle
	// an aliased kind that points to a mapped kind:
	// e.g other_macro should use the go_library resolver here:
	//   # gazelle:map_kind my_go_library go_library //:foo.bzl
	//   # gazelle:alias_kind other_macro my_go_library
	for _, mappedKind := range mr.mappedKinds[pkgRel] {
		if mappedKind.KindName == ruleKind {
			ruleKind = mappedKind.FromKind
			break
		}
	}

	// If the underlying kind is different, we need to apply the inverse map_kind operation so that
	// we get the Resolver for the underlying kind, not the mapped or aliased one that we see in the
	// existing BUILD file.
	if ruleKind != r.Kind() {
		fromKindResolver := mr.builtins[ruleKind]
		if fromKindResolver == nil {
			return nil
		}
		return inverseMapKindResolver{
			fromKind: ruleKind,
			delegate: fromKindResolver,
		}
	}

	return mr.builtins[ruleKind]
}

// inverseMapKindResolver applies an inverse of the map_kind
// operations to provided rules. This enables language
// modules to remain ignorant of mapped kinds.
type inverseMapKindResolver struct {
	fromKind string
	delegate indexResolver
}

var _ indexResolver = (*inverseMapKindResolver)(nil)

func (imkr inverseMapKindResolver) Name() string {
	return imkr.delegate.Name()
}

func (imkr inverseMapKindResolver) Imports(ctx context.Context, args resolve.ImportsArgs) (resolve.ImportsResult, error) {
	args.Rule = imkr.inverseMapKind(args.Rule)
	return imkr.delegate.Imports(ctx, args)
}

func (imkr inverseMapKindResolver) Resolve(ctx context.Context, args resolve.ResolveArgs) error {
	args.Rule = imkr.inverseMapKind(args.Rule)
	return imkr.delegate.Resolve(ctx, args)
}

func (imkr inverseMapKindResolver) inverseMapKind(r *rule.Rule) *rule.Rule {
	rCopy := *r
	rCopy.SetKind(imkr.fromKind)
	return &rCopy
}
