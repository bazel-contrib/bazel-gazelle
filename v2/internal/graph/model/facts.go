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
	"fmt"
	"strings"
)

// Facts is a write-only sink extensions use to contribute facts about the
// package Gazelle is processing. Every fact's origin node must be a target
// in that package; otherwise the method returns an error wrapping
// ErrOutOfPackage.
type Facts interface {
	// Requires records that from imports imp. Linking later turns it into
	// DependsOn edges to the import's providers, in attribute r.Attr.
	Requires(from NodeKey, imp ImportSpec, r Requires) error

	// Provides records that from can be imported as imp.
	Provides(from NodeKey, imp ImportSpec) error

	// Embeds records that from embeds to.
	Embeds(from, to NodeKey) error

	// DependsOn records a dependency that doesn't come from an import, like
	// a fixed toolchain or runtime dependency.
	DependsOn(from, to NodeKey, d DependsOn) error

	// Edge records an extension-defined edge. Its kind must be registered
	// and start with the contributing extension's name.
	Edge(from, to NodeKey, e ExtensionEdge) error
}

// PackageFacts returns a Facts sink for extension lang contributing facts
// about package rel of the main repository. Facts are added with origin o.
//
// This is used by Gazelle; extensions receive the sink in their arguments.
func (b *Builder) PackageFacts(lang, rel string, o Origin) Facts {
	return &packageFacts{b: b, lang: lang, rel: rel, origin: o}
}

type packageFacts struct {
	b      *Builder
	lang   string
	rel    string
	origin Origin
}

func (f *packageFacts) check(from NodeKey) error {
	info, err := ParseKey(from)
	if err != nil {
		return err
	}
	if info.Class != KindTarget || !info.IsMainRepo() || info.Label.Pkg != f.rel {
		return fmt.Errorf("%w: extension %q processing package %q added a fact about %s", ErrOutOfPackage, f.lang, f.rel, from)
	}
	return nil
}

func checkImport(imp ImportSpec) error {
	if imp.Lang == "" || imp.Imp == "" || strings.Contains(imp.Lang, ":") {
		return fmt.Errorf("%w: import %+v: language must be non-empty and not contain ':', import must be non-empty", ErrNonCanonicalKey, imp)
	}
	return nil
}

func (f *packageFacts) Requires(from NodeKey, imp ImportSpec, r Requires) error {
	if err := f.check(from); err != nil {
		return err
	}
	if err := checkImport(imp); err != nil {
		return err
	}
	return f.b.AddEdge(from, ImportKey(imp), r, f.origin)
}

func (f *packageFacts) Provides(from NodeKey, imp ImportSpec) error {
	if err := f.check(from); err != nil {
		return err
	}
	if err := checkImport(imp); err != nil {
		return err
	}
	return f.b.AddEdge(from, ImportKey(imp), Provides{}, f.origin)
}

func (f *packageFacts) Embeds(from, to NodeKey) error {
	if err := f.check(from); err != nil {
		return err
	}
	return f.b.AddEdge(from, to, Embeds{}, f.origin)
}

func (f *packageFacts) DependsOn(from, to NodeKey, d DependsOn) error {
	if err := f.check(from); err != nil {
		return err
	}
	return f.b.AddEdge(from, to, d, f.origin)
}

func (f *packageFacts) Edge(from, to NodeKey, e ExtensionEdge) error {
	if err := f.check(from); err != nil {
		return err
	}
	if !strings.HasPrefix(string(e.Kind), f.lang+"/") {
		return fmt.Errorf("%w: extension %q may only add edges of kinds %q, got %q", ErrUnknownEdgeKind, f.lang, f.lang+"/*", e.Kind)
	}
	return f.b.AddEdge(from, to, e, f.origin)
}
