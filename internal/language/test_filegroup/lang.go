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

// Package test_filegroup generates an "all_files" filegroup target
// in each package. This target globs files in the same package and
// depends on subpackages.
//
// These rules are used for testing with go_bazel_test.
//
// This extension is experimental and subject to change. It is not included
// in the default Gazelle binary.
package test_filegroup

import (
	"context"
	"path"

	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

const testFilegroupName = "test_filegroup"

type testFilegroupLang struct {
	Started, Resolved, Finished bool
}

func NewV2() language.Language {
	return &testFilegroupLang{}
}

var _ language.Generator = (*testFilegroupLang)(nil)
var _ language.OnStarter = (*testFilegroupLang)(nil)
var _ language.OnResolver = (*testFilegroupLang)(nil)
var _ language.OnFinisher = (*testFilegroupLang)(nil)
var _ resolve.Resolver = (*testFilegroupLang)(nil)

func (*testFilegroupLang) Name() string { return testFilegroupName }

func (*testFilegroupLang) Kinds() []rule.KindInfo {
	return kinds
}

var kinds = []rule.KindInfo{{
	Name:           "filegroup",
	NonEmptyAttrs:  map[string]bool{"srcs": true, "deps": true},
	MergeableAttrs: map[string]bool{"srcs": true},
}}

func (l *testFilegroupLang) OnStart(ctx context.Context) error {
	l.Started = true
	return nil
}

func (l *testFilegroupLang) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	if !l.Started {
		panic("Generate must not be called before OnStart")
	}
	if l.Resolved {
		panic("Generate must not be called after OnResolve")
	}

	r := rule.NewRule("filegroup", "all_files")
	srcs := make([]string, 0, len(args.Subdirs)+len(args.RegularFiles))
	srcs = append(srcs, args.RegularFiles...)
	for _, f := range args.Subdirs {
		pkg := path.Join(args.Rel, f)
		srcs = append(srcs, "//"+pkg+":all_files")
	}
	r.SetAttr("srcs", srcs)
	r.SetAttr("testonly", true)
	if args.File == nil || !args.File.HasDefaultVisibility() {
		r.SetAttr("visibility", []string{"//visibility:public"})
	}
	return language.GenerateResult{
		Gen:     []*rule.Rule{r},
		Imports: []any{nil},
	}, nil
}

func (l *testFilegroupLang) OnResolve(ctx context.Context) error {
	l.Resolved = true
	return nil
}

func (l *testFilegroupLang) Resolve(ctx context.Context, args resolve.ResolveArgs) error {
	if !l.Resolved {
		panic("Expected a call to OnResolve before Resolve")
	}
	if l.Finished {
		panic("Resolve must be called before calling OnFinish")
	}
	return nil
}

func (l *testFilegroupLang) OnFinish(ctx context.Context) error {
	l.Finished = true
	return nil
}
