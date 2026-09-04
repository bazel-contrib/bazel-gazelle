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

package main

import (
	"strings"
	"testing"
)

func TestUniquePackageName(t *testing.T) {
	counts := map[string]int{
		"language": 1,
		"compat":   1,
	}
	got := []string{
		uniquePackageName("proto", counts),
		uniquePackageName("proto", counts),
		uniquePackageName("proto_2", counts),
		uniquePackageName("proto", counts),
		uniquePackageName("language", counts),
	}
	want := []string{"proto", "proto_2", "proto_2_2", "proto_3", "language_2"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("uniquePackageName call %d: got %q, want %q", i+1, got[i], want[i])
		}
	}
}

func TestGenerateOutputReservedPackageName(t *testing.T) {
	out := string(generateOutput([]extension{{
		ImportPath: "example.com/language",
		PkgName:    "language",
		LocalName:  "language_2",
		FuncName:   "NewLanguage",
	}}))

	if !strings.Contains(out, `language_2 "example.com/language"`) {
		t.Errorf("output missing aliased import:\n%s", out)
	}
	if !strings.Contains(out, "language_2.NewLanguage(),") {
		t.Errorf("output missing aliased call:\n%s", out)
	}
}

func TestGenerateOutputWithCompat(t *testing.T) {
	out := string(generateOutput([]extension{{
		ImportPath: "example.com/foo",
		PkgName:    "foo",
		LocalName:  "foo",
		FuncName:   "NewLanguage",
		CompatWrap: true,
	}}))

	for _, want := range []string{
		`"github.com/bazel-contrib/bazel-gazelle/v2/language"`,
		`"github.com/bazel-contrib/bazel-gazelle/v2/compat"`,
		"compat.LanguageV2(foo.NewLanguage()),",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestGenerateOutputNative(t *testing.T) {
	out := string(generateOutput([]extension{{
		ImportPath: "example.com/foo",
		PkgName:    "foo",
		LocalName:  "foo",
		FuncName:   "NewV2",
	}}))

	if strings.Contains(out, "compat") {
		t.Errorf("native v2 output should not import compat:\n%s", out)
	}
	if !strings.Contains(out, "foo.NewV2(),") {
		t.Errorf("output missing foo.NewV2():\n%s", out)
	}
}
