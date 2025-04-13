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

package golang

import (
	"go/build/constraint"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFilterBuildTags(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		input constraint.Expr
		want  constraint.Expr
	}{
		{
			desc:  "should remain",
			input: mustParseBuildTag(t, "go1.8 || go1.9"),
			want:  mustParseBuildTag(t, "go1.8 || go1.9"),
		},
		{
			desc:  "simple 1",
			input: mustParseBuildTag(t, "!(go1.8 || go1.9)"),
			want:  mustParseBuildTag(t, "go1.8 && go1.9"),
		},
		{
			desc:  "simple 2",
			input: mustParseBuildTag(t, "!(foobar || go1.8 || go1.9)"),
			want:  mustParseBuildTag(t, "!foobar && go1.8 && go1.9"),
		},
		{
			desc:  "complex 1",
			input: mustParseBuildTag(t, "!(cgo && (go1.8 || go1.9) || race || msan)"),
			want:  mustParseBuildTag(t, "(cgo || (go1.8 && go1.9)) && race && msan"),
		},
		{
			desc:  "complex 2",
			input: mustParseBuildTag(t, "!(cgo && (go1.8 || go1.9 && (race && foobar)))"),
			want:  mustParseBuildTag(t, "cgo || go1.8 && (go1.9 || (race || !foobar))"),
		},
		{
			desc:  "complex 3",
			input: mustParseBuildTag(t, "!(cgo && (go1.8 || go1.9 && (race && foobar) || baz))"),
			want:  mustParseBuildTag(t, "cgo || (go1.8 && (go1.9 || (race || !foobar)) && !baz)"),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			bt := newBuildTags(tc.input)
			if diff := cmp.Diff(tc.want, bt.expr); diff != "" {
				t.Errorf("(-want, +got): %s", diff)
			}
		})
	}
}
