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
	"context"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/merger"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

// TODO: Rename this extension now that it handles multiple package() attributes.
type visibilityExtension struct{}

var (
	_ language.Language  = (*visibilityExtension)(nil)
	_ config.Configurer  = (*visibilityExtension)(nil)
	_ language.Generator = (*visibilityExtension)(nil)
)

// NewV2 constructs a new language.Language modifying visibility.
func NewV2() language.Language {
	return &visibilityExtension{}
}

func (*visibilityExtension) Name() string {
	return _extName
}

// Kinds instructs gazelle to match any 'package' rule as BUILD files can only have one.
func (*visibilityExtension) Kinds() []rule.KindInfo {
	return []rule.KindInfo{{
		Name:     "package",
		MatchAny: true,
		MergeableAttrs: map[string]bool{
			"default_visibility": true,
			"features":           true,
		},
	}}
}

// Generate sets default_visibility and features on package() when configured.
func (*visibilityExtension) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	res := language.GenerateResult{}
	cfg := getVisConfig(args.Config)

	if len(cfg.visibilityTargets) == 0 && len(cfg.features) == 0 {
		return res, nil
	}

	if args.File == nil {
		// No need to create a visibility if we're not in a visible directory.
		return res, nil
	}

	r := rule.NewRule("package", "")
	for _, er := range args.File.Rules {
		if er.Kind() == "package" {
			if vis := er.Attr("default_visibility"); vis != nil {
				r.SetAttr("default_visibility", vis)
			}
			if feat := er.Attr("features"); feat != nil {
				r.SetAttr("features", feat)
			}
			break
		}
	}

	if len(cfg.visibilityTargets) > 0 {
		r.SetAttr("default_visibility", cfg.visibilityTargets)
	}
	if len(cfg.features) > 0 {
		r.SetAttr("features", cfg.features)
	}

	// Start after the first statements if no rules exist
	insertIndex := len(args.File.File.Stmt)
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
	return res, nil
}
