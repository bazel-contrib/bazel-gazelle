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
	"context"

	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

var _ language.Language = (*xlang)(nil)
var _ language.Generator = (*xlang)(nil)

type xlang struct{}

func NewV2() language.Language {
	return &xlang{}
}

func (x *xlang) Name() string {
	return "x"
}

func (x *xlang) Kinds() []rule.KindInfo {
	return []rule.KindInfo{{Name: "x_library"}}
}

func (x *xlang) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	return language.GenerateResult{
		Gen:     []*rule.Rule{rule.NewRule("x_library", "x_default_library")},
		Imports: []any{nil},
	}, nil
}
