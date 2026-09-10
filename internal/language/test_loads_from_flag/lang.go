/* Copyright 2022 The Bazel Authors. All rights reserved.

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

// Package test_loads_from_flag generates a rule using a load location
// configured with a command-line flag.
//
// These rules are used for testing with gazelle_generation_test.
//
// This extension is experimental and subject to change. It is not included
// in the default Gazelle binary.
package test_loads_from_flag

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/compat"
	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

const testLoadsFromFlagName = "test_loads_from_flag"

type testLoadsFromFlag struct {
	load Load
}

var (
	_ language.Language     = (*testLoadsFromFlag)(nil)
	_ language.Generator    = (*testLoadsFromFlag)(nil)
	_ compat.FlagConfigurer = (*testLoadsFromFlag)(nil)
	_ compat.ApparentLoader = (*testLoadsFromFlag)(nil)
)

func NewV2() language.Language {
	return &testLoadsFromFlag{}
}

func (l *testLoadsFromFlag) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	fs.Var(&l.load, "custom-load", "repo,symbol")
}

func (l *testLoadsFromFlag) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	c.Exts[testLoadsFromFlagName] = l.load
	return nil
}

func (*testLoadsFromFlag) Name() string { return testLoadsFromFlagName }

func (l *testLoadsFromFlag) Kinds() []rule.KindInfo {
	return []rule.KindInfo{{Name: l.load.symbol}}
}

func (l *testLoadsFromFlag) ApparentLoads(func(string) string) []rule.LoadInfo {
	if l.load.from == "" {
		return nil
	}
	return []rule.LoadInfo{{
		Name:    l.load.from,
		Symbols: []string{l.load.symbol},
	}}
}

func (*testLoadsFromFlag) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	load := args.Config.Exts[testLoadsFromFlagName].(Load)
	r := rule.NewRule(load.symbol, "gen")
	return language.GenerateResult{
		Gen:     []*rule.Rule{r},
		Imports: []any{nil},
	}, nil
}

type Load struct {
	from   string
	symbol string
}

func (l *Load) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return fmt.Errorf("want exactly one comma")
	}
	l.from = parts[0]
	l.symbol = parts[1]
	return nil
}

func (l *Load) String() string {
	return fmt.Sprintf("load(%q, %q)", l.from, l.symbol)
}
