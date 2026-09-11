/* Copyright 2023 The Bazel Authors. All rights reserved.

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

// Package `test_load_for_packed_rules` generates packed
// rule of `selects.config_setting_group`.
//
// This extension is experimental and subject to change. It is not included
// in the default Gazelle binary.
package test_load_for_packed_rules

import (
	"context"

	"github.com/bazel-contrib/bazel-gazelle/v2/compat"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

const testLoadForPackedRulesName = "test_load_for_packed_rules"

type testLoadForPackedRulesLang struct {
	Started, Resolved, Finished bool
}

var (
	_ language.Language     = (*testLoadForPackedRulesLang)(nil)
	_ language.Generator    = (*testLoadForPackedRulesLang)(nil)
	_ language.OnStarter    = (*testLoadForPackedRulesLang)(nil)
	_ language.OnResolver   = (*testLoadForPackedRulesLang)(nil)
	_ language.OnFinisher   = (*testLoadForPackedRulesLang)(nil)
	_ resolve.Resolver      = (*testLoadForPackedRulesLang)(nil)
	_ compat.ApparentLoader = (*testLoadForPackedRulesLang)(nil)
)

func NewV2() language.Language {
	return &testLoadForPackedRulesLang{}
}

var kinds = []rule.KindInfo{{
	Name: "selects.config_setting_group",
	NonEmptyAttrs: map[string]bool{"name": true},
	MergeableAttrs: map[string]bool{
		"match_all": true,
		"match_any": true,
	},
}}

func (*testLoadForPackedRulesLang) Name() string {
	return testLoadForPackedRulesName
}

func (*testLoadForPackedRulesLang) Kinds() []rule.KindInfo {
	return kinds
}

func (*testLoadForPackedRulesLang) ApparentLoads(func(string) string) []rule.LoadInfo {
	return []rule.LoadInfo{{
		Name:    "@bazel_skylib//lib:selects.bzl",
		Symbols: []string{"selects"},
	}}
}

func (l *testLoadForPackedRulesLang) OnStart(ctx context.Context) error {
	l.Started = true
	return nil
}

func (l *testLoadForPackedRulesLang) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	if !l.Started {
		panic("Generate must not be called before OnStart")
	}
	if l.Resolved {
		panic("Generate must not be called after OnResolve")
	}

	r := rule.NewRule("selects.config_setting_group", "all_configs_group")

	match := []string{
		"//:config_a",
		"//:config_b",
	}

	r.SetAttr("match_all", match)

	return language.GenerateResult{
		Gen:     []*rule.Rule{r},
		Imports: []any{nil},
	}, nil
}

func (l *testLoadForPackedRulesLang) OnResolve(ctx context.Context) error {
	l.Resolved = true
	return nil
}

func (l *testLoadForPackedRulesLang) Resolve(ctx context.Context, args resolve.ResolveArgs) error {
	if !l.Resolved {
		panic("Expected a call to OnResolve before Resolve")
	}
	if l.Finished {
		panic("Resolve must be called before calling OnFinish")
	}
	return nil
}

func (l *testLoadForPackedRulesLang) OnFinish(ctx context.Context) error {
	l.Finished = true
	return nil
}
