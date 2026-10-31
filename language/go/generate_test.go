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

package golang

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/compat"
	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/merger"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/bazel-contrib/bazel-gazelle/v2/walk"
	"github.com/bazelbuild/bazel-gazelle/language/proto"
	bzl "github.com/bazelbuild/buildtools/build"
	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/google/go-cmp/cmp"
)

func TestGenerateRules(t *testing.T) {
	testdataDir := "testdata"
	if runtime.GOOS == "windows" {
		var err error
		testdataDir, err = bazel.NewTmpDir("testdata")
		if err != nil {
			t.Fatal(err)
		}
		files, _ := bazel.ListRunfiles()
		parent := "language/go/testdata"
		for _, rf := range files {
			rel, err := filepath.Rel(parent, rf.ShortPath)
			if err != nil {
				continue
			}
			if strings.HasPrefix(rel, "..") {
				// make sure we're not moving around file that we're not interested in
				continue
			}
			newPath := filepath.FromSlash(path.Join(testdataDir, rel))
			if err := os.MkdirAll(filepath.FromSlash(filepath.Dir(newPath)), os.ModePerm); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(filepath.FromSlash(rf.Path), newPath); err != nil {
				t.Fatal(err)
			}
		}
	}

	c, langs, cexts := testConfig(
		t,
		"-build_file_name=BUILD.old",
		"-go_prefix=example.com/repo",
		"-repo_root="+testdataDir)

	// runfiles are symbolic links, which we need Walk to follow.
	content := []byte(`
# gazelle:follow **
`)
	f, err := rule.LoadData(filepath.FromSlash("BUILD.config"), "config", content)
	if err != nil {
		t.Fatal(err)
	}
	for _, cext := range cexts {
		configure(t, cext, c, "", f)
	}

	loads := loadsForTest(c, langs)
	var testsFound int
	cache := walk.NewCache(c)
	err = walk.Walk(t.Context(), c, cexts, cache, []string{testdataDir}, walk.VisitAllUpdateSubdirsMode, func(args walk.WalkFuncArgs) (walk.WalkFuncResult, error) {
		dir := args.Dir
		rel := args.Rel
		c := args.Config
		oldFile := args.File
		subdirs := args.Subdirs
		regularFiles := args.RegularFiles
		genFiles := args.GenFiles
		t.Run(rel, func(t *testing.T) {
			var empty, gen []*rule.Rule
			for _, lang := range langs {
				res := generate(t, lang, language.GenerateArgs{
					Config:       c,
					Dir:          dir,
					Rel:          rel,
					File:         oldFile,
					Subdirs:      subdirs,
					RegularFiles: regularFiles,
					GenFiles:     genFiles,
					OtherEmpty:   empty,
					OtherGen:     gen,
					Cache:        cache,
				})
				empty = append(empty, res.Empty...)
				gen = append(gen, res.Gen...)
			}
			isTest := false
			for _, name := range regularFiles {
				if name == "BUILD.want" {
					isTest = true
					break
				}
			}
			if !isTest {
				// GenerateRules may have side effects, so we need to run it, even if
				// there's no test.
				return
			}
			testsFound += 1
			f := rule.EmptyFile("test", "")
			for _, r := range gen {
				r.Insert(f)
			}
			convertImportsAttrs(f)
			merger.NewLoadFixer(loads).Fix(f)
			f.Sync()
			got := string(bzl.Format(f.File))
			wantPath := filepath.Join(dir, "BUILD.want")
			wantBytes, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("error reading %s: %v", wantPath, err)
			}
			want := string(wantBytes)
			want = strings.ReplaceAll(want, "\r\n", "\n")
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(-want, +got): %s", diff)
			}
		})
		return walk.WalkFuncResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Avoid spurious success if we fail to find any tests.
	if testsFound == 0 {
		t.Error("No rule generation tests were found")
	}
}

func TestGenerateRulesEmpty(t *testing.T) {
	c, langs, _ := testConfig(t, "-go_prefix=example.com/repo")
	goLang := langs[1].(*goLang)
	cache := walk.NewCache(c)
	res, err := goLang.Generate(t.Context(), language.GenerateArgs{
		Config: c,
		Dir:    "./foo",
		Rel:    "foo",
		Cache:  cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Gen) > 0 {
		t.Errorf("got %d generated rules; want 0", len(res.Gen))
	}
	f := rule.EmptyFile("test", "")
	for _, r := range res.Empty {
		r.Insert(f)
	}
	f.Sync()
	got := strings.TrimSpace(string(bzl.Format(f.File)))
	want := strings.TrimSpace(`
filegroup(name = "go_default_library_protos")

go_proto_library(name = "foo_go_proto")

go_library(name = "foo")

go_binary(name = "foo")

go_test(name = "foo_test")
`)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Test that no data attribute is added for an empty testdata subdirectory
func TestGenerateRulesEmptyTestdata(t *testing.T) {
	dir, err := bazel.NewTmpDir("example")
	if err != nil {
		t.Fatal(err)
	}

	// Create an empty testdata subdirectory
	testdataDir := filepath.Join(dir, "testdata")
	if err := os.Mkdir(testdataDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test file in the example directory
	testFile := filepath.Join(dir, "example_test.go")
	testContent := []byte(`package example

import "testing"

func TestExample(t *testing.T) {
	t.Log("test")
}
`)
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	c, langs, cexts := testConfig(t, "-go_prefix=example.com/repo", "-repo_root="+dir)
	goLang := langs[1].(*goLang)

	cache := walk.NewCache(c)
	err = walk.Walk(t.Context(), c, cexts, cache, []string{dir}, walk.VisitAllUpdateSubdirsMode, func(args walk.WalkFuncArgs) (walk.WalkFuncResult, error) {
		res, err := goLang.Generate(t.Context(), language.GenerateArgs{
			Config:       c,
			Dir:          dir,
			Rel:          "",
			Subdirs:      []string{"testdata"},
			RegularFiles: []string{"example_test.go"},
			Cache:        cache,
		})
		if err != nil {
			t.Fatal(err)
		}

		// Find the go_test rule
		var testRule *rule.Rule
		for _, r := range res.Gen {
			if r.Kind() == "go_test" {
				testRule = r
				break
			}
		}

		if testRule == nil {
			t.Fatal("expected a go_test rule to be generated")
		}

		// Verify that no data attribute was added
		if data := testRule.Attr("data"); data != nil {
			t.Errorf("expected no data attribute for empty testdata subdirectory, but got: %v", data)
		}
		return walk.WalkFuncResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerateRulesEmptyLegacyProto(t *testing.T) {
	c, langs, _ := testConfig(t, "-proto=legacy")
	goLang := langs[len(langs)-1].(*goLang)
	cache := walk.NewCache(c)
	res, err := goLang.Generate(t.Context(), language.GenerateArgs{
		Config: c,
		Dir:    "./foo",
		Rel:    "foo",
		Cache:  cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Empty {
		if kind := e.Kind(); kind == "proto_library" || kind == "go_proto_library" {
			t.Errorf("deleted rule %s ; should not delete in legacy proto mode", kind)
		}
	}
}

func TestGenerateRulesEmptyPackageProto(t *testing.T) {
	c, langs, _ := testConfig(t, "-proto=package", "-go_prefix=example.com/repo")
	cache := walk.NewCache(c)
	oldContent := []byte(`
proto_library(
    name = "dead_proto",
    srcs = ["dead.proto"],
)
`)
	old, err := rule.LoadData("BUILD.bazel", "", oldContent)
	if err != nil {
		t.Fatal(err)
	}
	var empty []*rule.Rule
	for _, lang := range langs {
		res := generate(t, lang, language.GenerateArgs{
			Config:     c,
			Dir:        "./foo",
			Rel:        "foo",
			File:       old,
			OtherEmpty: empty,
			Cache:      cache,
		})
		empty = append(empty, res.Empty...)
	}
	f := rule.EmptyFile("test", "")
	for _, r := range empty {
		r.Insert(f)
	}
	f.Sync()
	got := strings.TrimSpace(string(bzl.Format(f.File)))
	want := strings.TrimSpace(`
proto_library(name = "dead_proto")

go_proto_library(name = "dead_go_proto")

filegroup(name = "go_default_library_protos")

go_proto_library(name = "foo_go_proto")

go_library(name = "foo")

go_binary(name = "foo")

go_test(name = "foo_test")
`)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateRulesPrebuiltGoProtoRules(t *testing.T) {
	for _, protoFlag := range []string{
		"-proto=default",
		"-proto=package",
	} {
		t.Run("with flag: "+protoFlag, func(t *testing.T) {
			c, langs, _ := testConfig(t, protoFlag)
			cache := walk.NewCache(c)
			goLang := langs[len(langs)-1].(*goLang)

			res, err := goLang.Generate(t.Context(), language.GenerateArgs{
				Config:   c,
				Dir:      "./foo",
				Rel:      "foo",
				OtherGen: prebuiltProtoRules(),
				Cache:    cache,
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(res.Gen) != 0 {
				t.Errorf("got %d generated rules; want 0", len(res.Gen))
			}
			f := rule.EmptyFile("test", "")
			for _, r := range res.Gen {
				r.Insert(f)
			}
			f.Sync()
			got := strings.TrimSpace(string(bzl.Format(f.File)))
			want := strings.TrimSpace(`
		`)
			if got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// Test generated files that have been consumed by other rules should not be
// added to the go_default_library rule
func TestConsumedGenFiles(t *testing.T) {
	c := &config.Config{
		Exts: make(map[string]interface{}),
	}
	args := language.GenerateArgs{
		RegularFiles: []string{"regular.go"},
		GenFiles:     []string{"mocks.go"},
		Config:       c,
		Cache:        walk.NewCache(c),
	}
	otherRule := rule.NewRule("go_library", "go_mock_library")
	otherRule.SetAttr("srcs", []string{"mocks.go"})
	args.OtherGen = append(args.OtherGen, otherRule)

	gl := &goLang{
		goPkgRels: make(map[string]bool),
	}
	if err := gl.Configure(t.Context(), config.ConfigureArgs{Config: args.Config, Rel: "", File: nil}); err != nil {
		t.Fatal(err)
	}
	res, err := gl.Generate(t.Context(), args)
	if err != nil {
		t.Fatal(err)
	}
	got := res.Gen[0].AttrStrings("srcs")
	want := []string{"regular.go"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Test visibility attribute is only set if no default visibility is provided
// by the file or other rules.
func TestShouldSetVisibility(t *testing.T) {
	if !shouldSetVisibility(language.GenerateArgs{}) {
		t.Error("got 'False' for shouldSetVisibility with default args; expected 'True'")
	}

	if !shouldSetVisibility(language.GenerateArgs{
		File: rule.EmptyFile("path", "pkg"),
	}) {
		t.Error("got 'False' for shouldSetVisibility with empty file; expected 'True'")
	}

	fileWithDefaultVisibile, _ := rule.LoadData("path", "pkg", []byte(`package(default_visibility = "//src:__subpackages__")`))
	if shouldSetVisibility(language.GenerateArgs{
		File: fileWithDefaultVisibile,
	}) {
		t.Error("got 'True' for shouldSetVisibility with file with default visibility; expected 'False'")
	}

	defaultVisibilityRule := rule.NewRule("package", "")
	defaultVisibilityRule.SetAttr("default_visibility", []string{"//src:__subpackages__"})
	if shouldSetVisibility(language.GenerateArgs{
		OtherGen: []*rule.Rule{defaultVisibilityRule},
	}) {
		t.Error("got 'True' for shouldSetVisibility with rule defining a default visibility; expected 'False'")
	}
}

func prebuiltProtoRules() []*rule.Rule {
	protoRule := rule.NewRule("proto_library", "foo_proto")
	protoRule.SetAttr("srcs", []string{"foo.proto"})
	protoRule.SetAttr("visibility", []string{"//visibility:public"})
	protoRule.SetPrivateAttr(proto.PackageKey,
		proto.Package{
			Name: "foo",
			Files: map[string]proto.FileInfo{
				"foo.proto": {},
			},
			Imports: map[string]bool{},
			Options: map[string]string{},
		},
	)

	goProtoRule := rule.NewRule("go_proto_library", "foo_go_proto")
	goProtoRule.SetAttr("compilers", []string{"@io_bazel_rules_go//proto:go_proto"})
	goProtoRule.SetAttr("importpath", "hello/world/foo")
	goProtoRule.SetAttr("proto", ":foo_proto")
	protoRule.SetAttr("visibility", []string{"//visibility:public"})

	return []*rule.Rule{protoRule, goProtoRule}
}

// convertImportsAttrs copies private attributes to regular attributes, which
// will later be written out to build files. This allows tests to check the
// values of private attributes with simple string comparison.
func convertImportsAttrs(f *rule.File) {
	for _, r := range f.Rules {
		v := r.PrivateAttr(gazelleImportsKey)
		if v != nil {
			r.SetAttr(gazelleImportsKey, v)
		}
	}
}

func TestProtoLibraryCompilers(t *testing.T) {
	for _, tc := range []struct {
		desc          string
		moduleContent string
		rulesGoName   string
	}{
		{
			desc:        "workspace_fallback",
			rulesGoName: "io_bazel_rules_go",
		},
		{
			desc: "bzlmod_default_apparent_name",
			moduleContent: `bazel_dep(name = "rules_go", version = "0.60.0")
`,
			rulesGoName: "rules_go",
		},
		{
			desc: "bzlmod_custom_repo_name",
			moduleContent: `bazel_dep(name = "rules_go", version = "0.60.0", repo_name = "my_rules_go")
`,
			rulesGoName: "my_rules_go",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dir := t.TempDir()
			if tc.moduleContent != "" {
				if err := os.WriteFile(filepath.Join(dir, "MODULE.bazel"), []byte(tc.moduleContent), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			svcDir := filepath.Join(dir, "svc")
			if err := os.MkdirAll(svcDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(svcDir, "svc.proto"), []byte(`
syntax = "proto2";
option go_package = "example.com/repo/svc";
service S {}
`), 0o644); err != nil {
				t.Fatal(err)
			}

			c, langs, cexts := testConfig(t,
				"-go_prefix=example.com/repo",
				"-repo_root="+dir)
			for _, cext := range cexts {
				configure(t, cext, c, "", nil)
			}

			var got []string
			cache := walk.NewCache(c)
			err := walk.Walk(t.Context(), c, cexts, cache, []string{dir}, walk.VisitAllUpdateSubdirsMode, func(args walk.WalkFuncArgs) (walk.WalkFuncResult, error) {
				var empty, gen []*rule.Rule
				for _, lang := range langs {
					res := generate(t, lang, language.GenerateArgs{
						Config:       args.Config,
						Dir:          args.Dir,
						Rel:          args.Rel,
						File:         args.File,
						Subdirs:      args.Subdirs,
						RegularFiles: args.RegularFiles,
						GenFiles:     args.GenFiles,
						OtherEmpty:   empty,
						OtherGen:     gen,
						Cache:        cache,
					})
					empty = append(empty, res.Empty...)
					gen = append(gen, res.Gen...)
				}
				for _, r := range gen {
					if r.Kind() == "go_proto_library" {
						got = r.AttrStrings("compilers")
					}
				}
				return walk.WalkFuncResult{}, nil
			})
			if err != nil {
				t.Fatal(err)
			}

			want := []string{
				fmt.Sprintf("@%s//proto:go_proto", tc.rulesGoName),
				fmt.Sprintf("@%s//proto:go_grpc_v2", tc.rulesGoName),
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(-want, +got):\n%s", diff)
			}
		})
	}
}

func TestPlatformSpecificCgo_EdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name      string
		files     map[string]string
		wantCgo   string
		wantSrcs  string
		wantCopts string
	}{
		{
			name: "objc_cpp_preprocessed_asm_and_pure_go_asm_with_headers",
			files: map[string]string{
				"BUILD.bazel":   "# gazelle:go_copts -Wall\n",
				"pure.go":       "package foo\n",
				"asm_linux.s":   "#include \"asm_const.h\"\n",
				"asm_const.h":   "#define CONST 1\n",
				"cgo_darwin.go": "//go:build darwin\n\npackage foo\n\nimport \"C\"\n",
				"bridge.m":      "// ObjC companion\n",
				"bridge_cxx.mm": "// ObjC++ companion\n",
				"bridge.hpp":    "// C++ header\n",
				"cgo_asm.S":     "// C-preprocessed asm\n",
				"simd_amd64.c":  "int simd(void) { return 0; }\n",
				"helper_unix.c": "//go:build unix\n\nint u(void) { return 0; }\n",
				"opt_in_icu.go": "//go:build icu\n\npackage foo\n\nimport \"C\"\n",
				"icu_linux.c":   "int icu(void) { return 0; }\n",
			},
			wantCgo: `select({
    "@io_bazel_rules_go//go/platform:darwin": True,
    "@io_bazel_rules_go//go/platform:ios": True,
    "//conditions:default": False,
})`,
			wantSrcs: `[
    "asm_const.h",
    "asm_linux.s",
    "cgo_darwin.go",
    "pure.go",
] + select({
    "@io_bazel_rules_go//go/platform:darwin": [
        "bridge.hpp",
        "bridge.m",
        "bridge_cxx.mm",
        "cgo_asm.S",
        "helper_unix.c",
    ],
    "@io_bazel_rules_go//go/platform:ios": [
        "bridge.hpp",
        "bridge.m",
        "bridge_cxx.mm",
        "cgo_asm.S",
        "helper_unix.c",
    ],
    "//conditions:default": [],
}) + select({
    "@io_bazel_rules_go//go/platform:darwin_amd64": [
        "simd_amd64.c",
    ],
    "@io_bazel_rules_go//go/platform:ios_amd64": [
        "simd_amd64.c",
    ],
    "//conditions:default": [],
})`,
			wantCopts: `select({
    "@io_bazel_rules_go//go/platform:darwin": [
        "-Wall",
    ],
    "@io_bazel_rules_go//go/platform:ios": [
        "-Wall",
    ],
    "//conditions:default": [],
})`,
		},
		{
			name: "multi_platform_os_plus_os_arch_disjoint_select",
			files: map[string]string{
				"pure.go":            "package foo\n",
				"cgo_darwin.go":      "//go:build darwin\n\npackage foo\n\nimport \"C\"\n",
				"cgo_linux_amd64.go": "//go:build linux && amd64\n\npackage foo\n\nimport \"C\"\n",
				"common.c":           "int common(void) { return 0; }\n",
			},
			wantCgo: `select({
    "@io_bazel_rules_go//go/platform:android_amd64": True,
    "@io_bazel_rules_go//go/platform:darwin_386": True,
    "@io_bazel_rules_go//go/platform:darwin_amd64": True,
    "@io_bazel_rules_go//go/platform:darwin_arm": True,
    "@io_bazel_rules_go//go/platform:darwin_arm64": True,
    "@io_bazel_rules_go//go/platform:ios_amd64": True,
    "@io_bazel_rules_go//go/platform:ios_arm64": True,
    "@io_bazel_rules_go//go/platform:linux_amd64": True,
    "//conditions:default": False,
})`,
			wantSrcs: `[
    "cgo_darwin.go",
    "cgo_linux_amd64.go",
    "pure.go",
] + select({
    "@io_bazel_rules_go//go/platform:android_amd64": [
        "common.c",
    ],
    "@io_bazel_rules_go//go/platform:darwin_386": [
        "common.c",
    ],
    "@io_bazel_rules_go//go/platform:darwin_amd64": [
        "common.c",
    ],
    "@io_bazel_rules_go//go/platform:darwin_arm": [
        "common.c",
    ],
    "@io_bazel_rules_go//go/platform:darwin_arm64": [
        "common.c",
    ],
    "@io_bazel_rules_go//go/platform:ios_amd64": [
        "common.c",
    ],
    "@io_bazel_rules_go//go/platform:ios_arm64": [
        "common.c",
    ],
    "@io_bazel_rules_go//go/platform:linux_amd64": [
        "common.c",
    ],
    "//conditions:default": [],
})`,
		},
		{
			name: "complementary_cgo_tags_collapse_to_true",
			files: map[string]string{
				"cgo_unix.go":  "//go:build unix\n\npackage foo\n\nimport \"C\"\n",
				"cgo_other.go": "//go:build !unix\n\npackage foo\n\nimport \"C\"\n",
				"helper.c":     "int h(void) { return 0; }\n",
			},
			wantCgo: `True`,
			wantSrcs: `[
    "cgo_other.go",
    "cgo_unix.go",
    "helper.c",
]`,
		},
		{
			name: "arch_constrained_cgo_and_companion_intersection",
			files: map[string]string{
				"pure.go":         "package foo\n",
				"cgo_amd64.go":    "package foo\n\nimport \"C\"\n",
				"generic.c":       "int g(void) { return 0; }\n",
				"helper_amd64.c":  "int a(void) { return 0; }\n",
				"unused_arm64.c":  "int u(void) { return 0; }\n",
				"helper_linux.c":  "int l(void) { return 0; }\n",
			},
			wantCgo: `select({
    "@io_bazel_rules_go//go/platform:amd64": True,
    "//conditions:default": False,
})`,
			wantSrcs: `[
    "cgo_amd64.go",
    "pure.go",
] + select({
    "@io_bazel_rules_go//go/platform:amd64": [
        "generic.c",
        "helper_amd64.c",
    ],
    "//conditions:default": [],
}) + select({
    "@io_bazel_rules_go//go/platform:android_amd64": [
        "helper_linux.c",
    ],
    "@io_bazel_rules_go//go/platform:linux_amd64": [
        "helper_linux.c",
    ],
    "//conditions:default": [],
})`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			c, langs, cexts := testConfig(t,
				"-go_prefix=example.com/foo",
				"-repo_root="+dir)
			for _, cext := range cexts {
				configure(t, cext, c, "", nil)
			}

			var libRule *rule.Rule
			cache := walk.NewCache(c)
			err := walk.Walk(t.Context(), c, cexts, cache, []string{dir}, walk.VisitAllUpdateSubdirsMode, func(args walk.WalkFuncArgs) (walk.WalkFuncResult, error) {
				for _, lang := range langs {
					res := generate(t, lang, language.GenerateArgs{
						Config:       args.Config,
						Dir:          args.Dir,
						Rel:          args.Rel,
						File:         args.File,
						Subdirs:      args.Subdirs,
						RegularFiles: args.RegularFiles,
						GenFiles:     args.GenFiles,
						Cache:        cache,
					})
					for _, r := range res.Gen {
						if r.Kind() == "go_library" {
							libRule = r
						}
					}
				}
				return walk.WalkFuncResult{}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if libRule == nil {
				t.Fatal("expected generated go_library rule")
			}

			gotCgo := bzl.FormatString(libRule.Attr("cgo"))
			if diff := cmp.Diff(tc.wantCgo, gotCgo); diff != "" {
				t.Errorf("cgo attr (-want, +got):\n%s", diff)
			}

			gotSrcs := bzl.FormatString(libRule.Attr("srcs"))
			if diff := cmp.Diff(tc.wantSrcs, gotSrcs); diff != "" {
				t.Errorf("srcs attr (-want, +got):\n%s", diff)
			}

			gotCopts := ""
			if copts := libRule.Attr("copts"); copts != nil {
				gotCopts = bzl.FormatString(copts)
			}
			if diff := cmp.Diff(tc.wantCopts, gotCopts); diff != "" {
				t.Errorf("copts attr (-want, +got):\n%s", diff)
			}
		})
	}
}

// TestPlatformSpecificCgo_MergedRules runs fix, generate, and merge on a
// directory with an existing build file, as the update command does. On every
// known platform, each merged rule must have cgo on where it has a C input,
// and srcs must not list a file twice. rules_go fails analysis otherwise.
func TestPlatformSpecificCgo_MergedRules(t *testing.T) {
	const cgoFile = "package foo\n\n/*\n#include <stdlib.h>\n*/\nimport \"C\"\n"
	const darwinCgo = `select({
    "@io_bazel_rules_go//go/platform:darwin": True,
    "@io_bazel_rules_go//go/platform:ios": True,
    "//conditions:default": False,
})`
	for _, tc := range []struct {
		desc string
		// build is the existing build file.
		build string
		// files are added to the source files of every case. Each case has
		// pure Go code, darwin-only cgo code, and a darwin-only C file.
		files map[string]string
		// wantCgo is the cgo attribute of the merged go_library.
		wantCgo string
	}{
		{
			desc: "upgrade_from_flat_cgo",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    importpath = "example.com/foo",
)
`,
			wantCgo: darwinCgo,
		},
		{
			desc: "keep_cdeps",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cdeps = [":c"],  # keep
    cgo = True,
    importpath = "example.com/foo",
)
`,
			wantCgo: "True",
		},
		{
			desc: "keep_clinkopts",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    clinkopts = ["-lm"],  # keep
    importpath = "example.com/foo",
)
`,
			wantCgo: "True",
		},
		{
			desc: "keep_c_src_in_flat_srcs",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "extra_darwin.c",  # keep
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    importpath = "example.com/foo",
)
`,
			wantCgo: "True",
		},
		{
			desc: "cdeps_select",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cdeps = select({
        "@io_bazel_rules_go//go/platform:linux": [":c"],
        "//conditions:default": [],
    }),
    cgo = True,
    importpath = "example.com/foo",
)
`,
			wantCgo: `select({
    "@io_bazel_rules_go//go/platform:darwin": True,
    "@io_bazel_rules_go//go/platform:ios": True,
    "@io_bazel_rules_go//go/platform:linux": True,
    "//conditions:default": False,
})`,
		},
		{
			desc: "kept_copts_value_in_select",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    copts = select({
        "@io_bazel_rules_go//go/platform:darwin": ["-DDARWIN"],
        "@io_bazel_rules_go//go/platform:linux_amd64": [
            "-DLINUX",  # keep
        ],
        "//conditions:default": [],
    }),
    importpath = "example.com/foo",
)
`,
			wantCgo: `select({
    "@io_bazel_rules_go//go/platform:darwin_386": True,
    "@io_bazel_rules_go//go/platform:darwin_amd64": True,
    "@io_bazel_rules_go//go/platform:darwin_arm": True,
    "@io_bazel_rules_go//go/platform:darwin_arm64": True,
    "@io_bazel_rules_go//go/platform:ios_amd64": True,
    "@io_bazel_rules_go//go/platform:ios_arm64": True,
    "@io_bazel_rules_go//go/platform:linux_amd64": True,
    "//conditions:default": False,
})`,
		},
		{
			// Merge cannot parse a select with other keys, so it keeps it.
			desc: "copts_select_with_other_keys",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    copts = select({
        ":opt": ["-O3"],
        "//conditions:default": [],
    }),
    importpath = "example.com/foo",
)
`,
			wantCgo: "True",
		},
		{
			desc: "kept_c_src_in_select",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "pure.go",
    ] + select({
        "@io_bazel_rules_go//go/platform:darwin": [
            "extra_darwin.c",  # keep
            "helper_darwin.c",
        ],
        "@io_bazel_rules_go//go/platform:ios": [
            "helper_darwin.c",
        ],
        "//conditions:default": [],
    }),
    cgo = select({
        "@io_bazel_rules_go//go/platform:darwin": True,
        "@io_bazel_rules_go//go/platform:ios": True,
        "//conditions:default": False,
    }),
    importpath = "example.com/foo",
)
`,
			wantCgo: darwinCgo,
		},
		{
			desc: "kept_cgo_file_with_undeclared_tag",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "opt_in_icu.go",  # keep
        "pure.go",
    ],
    cgo = True,
    importpath = "example.com/foo",
)
`,
			files:   map[string]string{"opt_in_icu.go": "//go:build icu\n\n" + cgoFile},
			wantCgo: "True",
		},
		{
			desc: "embedding_test_with_cdeps",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    cdeps = [":c"],
    embed = [":foo"],
)
`,
			files:   map[string]string{"foo_test.go": "package foo\n"},
			wantCgo: "True",
		},
		{
			desc: "embedding_test_with_own_cgo",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    cdeps = select({
        "@io_bazel_rules_go//go/platform:linux": [":c"],
        "//conditions:default": [],
    }),
    cgo = select({  # keep
        "@io_bazel_rules_go//go/platform:linux": True,
        "//conditions:default": False,
    }),
    embed = [":foo"],
)
`,
			files:   map[string]string{"foo_test.go": "package foo\n"},
			wantCgo: darwinCgo,
		},
		{
			desc: "embedding_test_with_cgo_false_and_cdeps",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    cdeps = [":c"],
    cgo = False,
    embed = [":foo"],
)
`,
			files:   map[string]string{"foo_test.go": "package foo\n"},
			wantCgo: "True",
		},
		{
			desc: "cdeps_arch_select",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cdeps = select({
        "@io_bazel_rules_go//go/platform:amd64": [":c"],
        "//conditions:default": [],
    }),
    cgo = True,
    importpath = "example.com/foo",
)
`,
			wantCgo: `select({
    "@io_bazel_rules_go//go/platform:android_amd64": True,
    "@io_bazel_rules_go//go/platform:darwin_386": True,
    "@io_bazel_rules_go//go/platform:darwin_amd64": True,
    "@io_bazel_rules_go//go/platform:darwin_arm": True,
    "@io_bazel_rules_go//go/platform:darwin_arm64": True,
    "@io_bazel_rules_go//go/platform:dragonfly_amd64": True,
    "@io_bazel_rules_go//go/platform:freebsd_amd64": True,
    "@io_bazel_rules_go//go/platform:illumos_amd64": True,
    "@io_bazel_rules_go//go/platform:ios_amd64": True,
    "@io_bazel_rules_go//go/platform:ios_arm64": True,
    "@io_bazel_rules_go//go/platform:linux_amd64": True,
    "@io_bazel_rules_go//go/platform:netbsd_amd64": True,
    "@io_bazel_rules_go//go/platform:openbsd_amd64": True,
    "@io_bazel_rules_go//go/platform:osx_amd64": True,
    "@io_bazel_rules_go//go/platform:plan9_amd64": True,
    "@io_bazel_rules_go//go/platform:qnx_amd64": True,
    "@io_bazel_rules_go//go/platform:solaris_amd64": True,
    "@io_bazel_rules_go//go/platform:windows_amd64": True,
    "//conditions:default": False,
})`,
		},
		{
			desc: "kept_rule",
			build: `# keep
go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cdeps = [":c"],
    cgo = True,
    importpath = "example.com/foo",
)
`,
			wantCgo: "True",
		},
		{
			desc: "inverted_select",
			build: `go_library(
    name = "foo",
    srcs = [
        "cgo_darwin.go",
        "cgo_posix.go",
        "helper_darwin.c",
        "pure.go",
    ],
    cgo = True,
    importpath = "example.com/foo",
)
`,
			files: map[string]string{"cgo_posix.go": "//go:build !windows\n\n" + cgoFile},
			wantCgo: `select({
    "@io_bazel_rules_go//go/platform:windows": False,
    "//conditions:default": True,
})`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"BUILD.bazel":     tc.build,
				"pure.go":         "package foo\n",
				"cgo_darwin.go":   cgoFile,
				"helper_darwin.c": "int helper(void) { return 0; }\n",
			}
			for name, content := range tc.files {
				files[name] = content
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			merged := updateForTest(t, dir)
			var lib *rule.Rule
			for _, r := range merged.Rules {
				if r.Kind() == "go_library" {
					lib = r
				}
			}
			if lib == nil {
				t.Fatalf("no go_library in merged file:\n%s", merged.Format())
			}
			gotCgo := ""
			if cgo := lib.Attr("cgo"); cgo != nil {
				gotCgo = bzl.FormatString(cgo)
			}
			if diff := cmp.Diff(tc.wantCgo, gotCgo); diff != "" {
				t.Errorf("merged cgo (-want, +got):\n%s", diff)
			}
			checkMergedCgoRules(t, merged)

			// A second run must not change the file.
			content := merged.Format()
			if err := os.WriteFile(filepath.Join(dir, "BUILD.bazel"), content, 0o644); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(content), string(updateForTest(t, dir).Format())); diff != "" {
				t.Errorf("second run changed the build file (-first, +second):\n%s", diff)
			}
		})
	}
}

// updateForTest runs fix, generate, and pre-resolve merge in dir, which must
// have a build file, and returns the merged file.
func updateForTest(t *testing.T, dir string) *rule.File {
	t.Helper()
	c, langs, cexts := testConfig(t, "-go_prefix=example.com/foo", "-repo_root="+dir)
	var merged *rule.File
	cache := walk.NewCache(c)
	err := walk.Walk(t.Context(), c, cexts, cache, []string{dir}, walk.VisitAllUpdateSubdirsMode, func(args walk.WalkFuncArgs) (walk.WalkFuncResult, error) {
		for _, lang := range langs {
			if err := compat.LanguageWithDefaults(lang).Fix(t.Context(), language.FixArgs{
				Config: args.Config,
				Rel:    args.Rel,
				File:   args.File,
				Cache:  cache,
			}); err != nil {
				t.Fatal(err)
			}
		}
		var empty, gen []*rule.Rule
		for _, lang := range langs {
			res := generate(t, lang, language.GenerateArgs{
				Config:       args.Config,
				Dir:          args.Dir,
				Rel:          args.Rel,
				File:         args.File,
				Subdirs:      args.Subdirs,
				RegularFiles: args.RegularFiles,
				GenFiles:     args.GenFiles,
				OtherEmpty:   empty,
				OtherGen:     gen,
				Cache:        cache,
			})
			empty = append(empty, res.Empty...)
			gen = append(gen, res.Gen...)
		}
		if err := merger.MergeFile(merger.MergeFileArgs{
			File:        args.File,
			Empty:       empty,
			Gen:         gen,
			Phase:       merger.PreResolve,
			GetKindInfo: func(r *rule.Rule) rule.KindInfo { return goKindsMap[r.Kind()] },
		}); err != nil {
			t.Fatal(err)
		}
		args.File.Sync()
		merged = args.File
		return walk.WalkFuncResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return merged
}

// checkMergedCgoRules checks the rules_go analysis rules for cgo on every
// known platform. When cgo is off, a rule must not have cdeps, C compiler or
// linker flags, or C and C++ sources. A rule gets cgo from its own cgo
// attribute or from a library it embeds, but not from both. It also checks
// that srcs lists each file once.
func checkMergedCgoRules(t *testing.T, f *rule.File) {
	t.Helper()
	rulesByName := make(map[string]*rule.Rule)
	for _, r := range f.Rules {
		rulesByName[r.Name()] = r
	}
	for _, r := range f.Rules {
		if !isGoRule(r.Kind()) {
			continue
		}
		for _, p := range rule.KnownPlatforms {
			cgo := cgoOnPlatform(r.Attr("cgo"), p)
			for _, embed := range r.AttrStrings("embed") {
				if lib := rulesByName[strings.TrimPrefix(embed, ":")]; lib != nil && cgoOnPlatform(lib.Attr("cgo"), p) {
					if cgo {
						t.Errorf("%s on %s: more than one library with cgo enabled", r.Name(), p)
					}
					cgo = true
				}
			}
			srcs := stringsOnPlatform(r.Attr("srcs"), p)
			seen := make(map[string]bool)
			for _, src := range srcs {
				if seen[src] {
					t.Errorf("%s on %s: srcs lists %s twice", r.Name(), p, src)
				}
				seen[src] = true
			}
			if cgo {
				continue
			}
			for _, attr := range []string{"cdeps", "clinkopts", "copts", "cppopts", "cxxopts"} {
				if values := stringsOnPlatform(r.Attr(attr), p); len(values) > 0 {
					t.Errorf("%s on %s: %s set without cgo: %v", r.Name(), p, attr, values)
				}
			}
			for _, src := range srcs {
				switch path.Ext(src) {
				case ".c", ".cc", ".cxx", ".cpp", ".hh", ".hpp", ".hxx":
					t.Errorf("%s on %s: source %s needs cgo", r.Name(), p, src)
				}
			}
		}
	}
}

// selectedCases returns the values of the select cases in dict that apply on
// platform p. An OS and architecture key is more specific than an OS key or an
// architecture key. A key that is not a rules_go platform may match on any
// platform, so its value always applies.
func selectedCases(dict *bzl.DictExpr, p rule.Platform) []bzl.Expr {
	const prefix = "@io_bazel_rules_go//go/platform:"
	var exact, partial, other []bzl.Expr
	var dflt bzl.Expr
	for _, kv := range dict.List {
		key := kv.Key.(*bzl.StringExpr).Value
		switch {
		case key == "//conditions:default":
			dflt = kv.Value
		case key == prefix+p.String():
			exact = append(exact, kv.Value)
		case key == prefix+p.OS || key == prefix+p.Arch:
			partial = append(partial, kv.Value)
		case strings.HasPrefix(key, prefix):
		default:
			other = append(other, kv.Value)
		}
	}
	values := exact
	if len(values) == 0 {
		values = partial
	}
	values = append(values, other...)
	if len(values) == 0 && dflt != nil {
		values = []bzl.Expr{dflt}
	}
	return values
}

// stringsOnPlatform returns the strings that e lists on platform p. Values
// that are not strings are returned in their formatted form.
func stringsOnPlatform(e bzl.Expr, p rule.Platform) []string {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *bzl.ListExpr:
		var values []string
		for _, v := range e.List {
			if s, ok := v.(*bzl.StringExpr); ok {
				values = append(values, s.Value)
			} else {
				values = append(values, bzl.FormatString(v))
			}
		}
		return values
	case *bzl.BinaryExpr:
		return append(stringsOnPlatform(e.X, p), stringsOnPlatform(e.Y, p)...)
	case *bzl.CallExpr:
		if len(e.List) != 1 {
			break
		}
		if dict, ok := e.List[0].(*bzl.DictExpr); ok {
			var values []string
			for _, v := range selectedCases(dict, p) {
				values = append(values, stringsOnPlatform(v, p)...)
			}
			return values
		}
	}
	return []string{bzl.FormatString(e)}
}

// cgoOnPlatform reports whether a cgo attribute is True on platform p.
func cgoOnPlatform(e bzl.Expr, p rule.Platform) bool {
	switch e := e.(type) {
	case *bzl.Ident:
		return e.Name == "True"
	case *bzl.CallExpr:
		if len(e.List) != 1 {
			break
		}
		if dict, ok := e.List[0].(*bzl.DictExpr); ok {
			for _, v := range selectedCases(dict, p) {
				if cgoOnPlatform(v, p) {
					return true
				}
			}
		}
	}
	return false
}
