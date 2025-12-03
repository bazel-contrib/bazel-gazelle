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

package visibility

import (
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/merger"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// TODO: Rename this extension now that it handles multiple package() attributes.
type visibilityExtension struct{}

// NewLanguage constructs a new language.Language modifying visibility.
func NewLanguage() language.Language {
	return &visibilityExtension{}
}

// Kinds instructs gazelle to match any 'package' rule as BUILD files can only have one.
func (*visibilityExtension) Kinds() map[string]rule.KindInfo {
	return map[string]rule.KindInfo{
		"package": {
			MatchAny: true,
			MergeableAttrs: map[string]bool{
				"default_visibility": true,
				"features":           true,
			},
		},
	}
}

func (*visibilityExtension) Loads() []rule.LoadInfo {
	panic("ApparentLoads should be called instead")
}

// ApparentLoads noops because there are no imports to add
func (*visibilityExtension) ApparentLoads(func(string) string) []rule.LoadInfo {
	return nil
}

// GenerateRules does the hard work of setting the default_visibility if a config exists.
func (*visibilityExtension) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	res := language.GenerateResult{}
	cfg := getVisConfig(args.Config)

	if len(cfg.visibilityTargets) == 0 && len(cfg.features) == 0 {
		return res
	}

	if args.File == nil {
		// No need to create a visibility if we're not in a visible directory.
		return res
	}

	r := rule.NewRule("package", "")
	for _, er := range args.File.Rules {
		if er.Kind() == "package" {
			r = er
			break
		}
	}

	if len(cfg.visibilityTargets) > 0 {
		r.SetAttr("default_visibility", cfg.visibilityTargets)
	}
	if len(cfg.features) > 0 {
		r.SetAttr("features", cfg.features)
	}

	insertIndex := 0
	for _, existingRule := range args.File.Rules {
		if existingRule.Kind() != "package" {
			insertIndex = existingRule.Index()
			break
		}
	}
	r.SetPrivateAttr(merger.UnstableInsertIndexKey, insertIndex)

	res.Gen = append(res.Gen, r)
	// we have to add a nil to Imports because there is length-matching validation with Gen.
	res.Imports = append(res.Imports, nil)
	return res
}

// Fix noop because there is nothing out there to fix yet
func (*visibilityExtension) Fix(c *config.Config, f *rule.File) {}
