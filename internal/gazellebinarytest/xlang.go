/* Copyright 2018 The Bazel Authors. All rights reserved.

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

// gazellebinarytest provides a minimal implementation of language.Language.
// This is used to verify that gazelle_binary builds plugins and runs them
// in the correct order.
package gazellebinarytest

import (
	"flag"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

var _ config.Configurer = (*xlang)(nil)

type xlang struct{}

func NewLanguage() language.Language {
	return &xlang{}
}

func (x *xlang) Name() string {
	return "x"
}

func (x *xlang) Kinds() map[string]rule.KindInfo {
	return map[string]rule.KindInfo{
		"x_library": {},
	}
}

func (x *xlang) Loads() []rule.LoadInfo {
	return nil
}

func (x *xlang) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
}

func (x *xlang) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}

func (x *xlang) KnownDirectives() []string {
	return nil
}

func (x *xlang) Configure(c *config.Config, rel string, f *rule.File) {
}

func (x *xlang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	return language.GenerateResult{
		Gen:     []*rule.Rule{rule.NewRule("x_library", "x_default_library")},
		Imports: []interface{}{nil},
	}
}

func (x *xlang) Fix(c *config.Config, f *rule.File) {
}

func (x *xlang) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	return nil
}

func (x *xlang) Embeds(r *rule.Rule, from label.Label) []label.Label {
	return nil
}

func (x *xlang) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports interface{}, from label.Label) {
}
