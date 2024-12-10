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

package main

import (
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// metaResolver provides a rule.Resolver for any rule.Rule.
type metaResolver struct {
	// builtins provides a map of the language kinds to their resolver.
	builtins map[string]resolve.Resolver

	// mappedKinds provides a list of replacements used by File.Pkg.
	mappedKinds map[string][]config.MappedKind

	// aliasedKinds provides a dict of configured wrapper macros for each package
	aliasedKinds map[string]map[string]string
}

func newMetaResolver() *metaResolver {
	return &metaResolver{
		builtins:     make(map[string]resolve.Resolver),
		mappedKinds:  make(map[string][]config.MappedKind),
		aliasedKinds: make(map[string]map[string]string),
	}
}

// AddBuiltin registers a builtin kind with its info.
func (mr *metaResolver) AddBuiltin(kindName string, resolver resolve.Resolver) {
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

// Resolver returns a resolver for the given rule and package, and a bool
// indicating whether one was found. Empty string may be passed for pkgRel,
// which results in consulting the builtin kinds only.
func (mr *metaResolver) Resolver(r *rule.Rule, pkgRel string) resolve.Resolver {
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
	delegate resolve.Resolver
}

var _ resolve.Resolver = (*inverseMapKindResolver)(nil)

func (imkr inverseMapKindResolver) Name() string {
	return imkr.delegate.Name()
}

func (imkr inverseMapKindResolver) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	r = imkr.inverseMapKind(r)
	return imkr.delegate.Imports(c, r, f)
}

func (imkr inverseMapKindResolver) Embeds(r *rule.Rule, from label.Label) []label.Label {
	r = imkr.inverseMapKind(r)
	return imkr.delegate.Embeds(r, from)
}

func (imkr inverseMapKindResolver) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports interface{}, from label.Label) {
	r = imkr.inverseMapKind(r)
	imkr.delegate.Resolve(c, ix, rc, r, imports, from)
}

func (imkr inverseMapKindResolver) inverseMapKind(r *rule.Rule) *rule.Rule {
	rCopy := *r
	rCopy.SetKind(imkr.fromKind)
	return &rCopy
}
