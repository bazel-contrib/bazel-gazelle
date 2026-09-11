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
	"fmt"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

func newExt(t *testing.T) *visibilityExtension {
	t.Helper()
	ext, ok := NewV2().(*visibilityExtension)
	if !ok {
		t.Fatal("NewV2() must return *visibilityExtension")
	}
	return ext
}

func TestExtensionMetadata(t *testing.T) {
	ext := newExt(t)

	if ext.KnownDirectives() == nil {
		t.Fatal("expected not nil")
	}
	if ext.Name() != "visibility" {
		t.Fatalf("Name() = %q, want %q", ext.Name(), "visibility")
	}
	if len(ext.Kinds()) != 1 {
		t.Fatal("expected one kind")
	}
}

func TestNoDirective(t *testing.T) {
	ctx := t.Context()
	cfg := config.New()
	file := rule.EmptyFile("path", "pkg")

	ext := newExt(t)
	if err := ext.Configure(ctx, config.ConfigureArgs{Config: cfg, Rel: "rel", File: file}); err != nil {
		t.Fatal(err)
	}
	res, err := ext.Generate(ctx, language.GenerateArgs{
		Config: cfg,
		File:   rule.EmptyFile("path/file", "pkg"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Imports) != 0 {
		t.Fatal("expected empty array")
	}
	if len(res.Gen) != 0 {
		t.Fatal("expected empty array")
	}
}

func TestNewDirective(t *testing.T) {
	ctx := t.Context()
	testVis := "//src:__subpackages__"
	cfg := config.New()
	file, err := rule.LoadData("path", "pkg", []byte(fmt.Sprintf("# gazelle:default_visibility %s", testVis)))
	if err != nil {
		t.Fatal("expected nil")
	}

	ext := newExt(t)
	if err := ext.Configure(ctx, config.ConfigureArgs{Config: cfg, Rel: "rel", File: file}); err != nil {
		t.Fatal(err)
	}
	res, err := ext.Generate(ctx, language.GenerateArgs{
		Config: cfg,
		File:   rule.EmptyFile("path/file", "pkg"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Gen) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res.Imports) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res.Gen[0].AttrStrings("default_visibility")) != 1 {
		t.Fatal("expected array of length 1")
	}
	if testVis != res.Gen[0].AttrStrings("default_visibility")[0] {
		t.Fatal("expected returned visibility to match 'testVis'")
	}
}

func TestReplacementDirective(t *testing.T) {
	ctx := t.Context()
	testVis := "//src:__subpackages__"
	cfg := config.New()
	file, err := rule.LoadData("path", "pkg", []byte(fmt.Sprintf(`
# gazelle:default_visibility %s

package(default_visibility = "//not-src:__subpackages__")
`, testVis)))
	if err != nil {
		t.Fatalf("expected not nil - %+v", err)
	}

	ext := newExt(t)
	if err := ext.Configure(ctx, config.ConfigureArgs{Config: cfg, Rel: "rel", File: file}); err != nil {
		t.Fatal(err)
	}
	res, err := ext.Generate(ctx, language.GenerateArgs{
		Config: cfg,
		File:   rule.EmptyFile("path/file", "pkg"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Gen) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res.Imports) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res.Gen[0].AttrStrings("default_visibility")) != 1 {
		t.Fatal("expected array of length 1")
	}
	if testVis != res.Gen[0].AttrStrings("default_visibility")[0] {
		t.Fatal("expected returned visibility to match '//src:__subpackages__'")
	}
}

func TestMultipleDirectives(t *testing.T) {
	ctx := t.Context()
	testVis1 := "//src:__subpackages__"
	testVis2 := "//src2:__subpackages__"
	cfg := config.New()
	file, err := rule.LoadData("path", "pkg", []byte(fmt.Sprintf(`
# gazelle:default_visibility %s
# gazelle:default_visibility %s
`, testVis1, testVis2)))
	if err != nil {
		t.Fatalf("expected not nil - %+v", err)
	}

	ext := newExt(t)
	if err := ext.Configure(ctx, config.ConfigureArgs{Config: cfg, Rel: "rel", File: file}); err != nil {
		t.Fatal(err)
	}
	res, err := ext.Generate(ctx, language.GenerateArgs{
		Config: cfg,
		File:   rule.EmptyFile("path/file", "pkg"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Gen) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res.Imports) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res.Gen[0].AttrStrings("default_visibility")) != 2 {
		t.Fatal("expected array of length 2")
	}
	if testVis1 != res.Gen[0].AttrStrings("default_visibility")[0] {
		t.Fatal("expected returned visibility to match '//src:__subpackages__'")
	}
	if testVis2 != res.Gen[0].AttrStrings("default_visibility")[1] {
		t.Fatal("expected returned visibility to match '//src2:__subpackages__'")
	}
}

func TestMultipleDefaultsSingleDirective(t *testing.T) {
	ctx := t.Context()
	testVis1 := "//src:__subpackages__"
	testVis2 := "//src2:__subpackages__"
	cfg := config.New()
	file, err := rule.LoadData("path", "pkg", []byte(fmt.Sprintf(`
# gazelle:default_visibility %s,%s
`, testVis1, testVis2)))
	if err != nil {
		t.Fatalf("expected not nil - %+v", err)
	}

	ext := newExt(t)
	if err := ext.Configure(ctx, config.ConfigureArgs{Config: cfg, Rel: "rel", File: file}); err != nil {
		t.Fatal(err)
	}
	res, err := ext.Generate(ctx, language.GenerateArgs{
		Config: cfg,
		File:   rule.EmptyFile("path/file", "pkg"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Gen) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res.Imports) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res.Gen[0].AttrStrings("default_visibility")) != 2 {
		t.Fatal("expected array of length 2")
	}
	if testVis1 != res.Gen[0].AttrStrings("default_visibility")[0] {
		t.Fatal("expected returned visibility to match '//src:__subpackages__'")
	}
	if testVis2 != res.Gen[0].AttrStrings("default_visibility")[1] {
		t.Fatal("expected returned visibility to match '//src2:__subpackages__'")
	}
}

func TestNoRuleIfNoBuildFile(t *testing.T) {
	ctx := t.Context()
	testVis1 := "//src:__subpackages__"
	cfg := config.New()
	file, err := rule.LoadData("path", "pkg", []byte(fmt.Sprintf(`
# gazelle:default_visibility %s
# gazelle:default_features foo
`, testVis1)))
	if err != nil {
		t.Fatalf("expected not nil - %+v", err)
	}

	ext := newExt(t)
	if err := ext.Configure(ctx, config.ConfigureArgs{Config: cfg, Rel: "rel", File: file}); err != nil {
		t.Fatal(err)
	}
	res, err := ext.Generate(ctx, language.GenerateArgs{
		Config: cfg,
		File:   nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Gen) != 0 {
		t.Fatal("expected array of length 0, no rules generated for missing BUILD.bazel file")
	}
	if len(res.Imports) != 0 {
		t.Fatal("expected array of length 0")
	}
}

func TestMultipleDirectivesAcrossFilesSupercede(t *testing.T) {
	ctx := t.Context()
	testVis1 := "//src:__subpackages__"
	testVis2 := "//src2:__subpackages__"
	file1, err := rule.LoadData("path", "pkg", []byte(fmt.Sprintf(`
# gazelle:default_visibility %s
`, testVis1)))
	if err != nil {
		t.Fatalf("expected not nil - %+v", err)
	}
	file2, err := rule.LoadData("path/path", "pkg", []byte(fmt.Sprintf(`
# gazelle:default_visibility %s
`, testVis2)))
	if err != nil {
		t.Fatalf("expected not nil - %+v", err)
	}

	cfg := config.New()
	ext := newExt(t)
	if err := ext.Configure(ctx, config.ConfigureArgs{Config: cfg, Rel: "path", File: file1}); err != nil {
		t.Fatal(err)
	}

	// clone the config as if we were decending through Walk
	cfg2 := cfg.Clone()
	if err := ext.Configure(ctx, config.ConfigureArgs{Config: cfg2, Rel: "path/path", File: file2}); err != nil {
		t.Fatal(err)
	}

	res2, err := ext.Generate(ctx, language.GenerateArgs{
		Config: cfg2,
		File:   rule.EmptyFile("path/path/file", "pkg"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res2.Gen) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res2.Imports) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res2.Gen[0].AttrStrings("default_visibility")) != 1 {
		t.Fatal("expected array of length 1")
	}
	if testVis2 != res2.Gen[0].AttrStrings("default_visibility")[0] {
		t.Fatal("expected returned visibility to match '//src2:__subpackages__'")
	}

	res1, err := ext.Generate(ctx, language.GenerateArgs{
		Config: cfg,
		File:   rule.EmptyFile("path/file", "pkg"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res1.Gen) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res1.Imports) != 1 {
		t.Fatal("expected array of length 1")
	}
	if len(res1.Gen[0].AttrStrings("default_visibility")) != 1 {
		t.Fatal("expected array of length 1")
	}
	if testVis1 != res1.Gen[0].AttrStrings("default_visibility")[0] {
		t.Fatal("expected returned visibility to match '//src:__subpackages__'")
	}
}
