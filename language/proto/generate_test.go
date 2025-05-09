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

package proto

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/merger"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/bazelbuild/bazel-gazelle/walk"

	bzl "github.com/bazelbuild/buildtools/build"
)

func TestGenerateRules(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO(jayconrod): set up testdata directory on windows before running test
		if _, err := os.Stat("testdata"); os.IsNotExist(err) {
			t.Skip("testdata missing on windows due to lack of symbolic links")
		} else if err != nil {
			t.Fatal(err)
		}
	}

	c, lang, _ := testConfig(t, "testdata")

	walk.Walk(c, []config.Configurer{lang}, []string{"testdata"}, walk.VisitAllUpdateSubdirsMode, func(dir, rel string, c *config.Config, update bool, oldFile *rule.File, subdirs, regularFiles, genFiles []string) {
		isTest := false
		for _, name := range regularFiles {
			if name == "BUILD.want" {
				isTest = true
				break
			}
		}
		if !isTest {
			return
		}
		t.Run(rel, func(t *testing.T) {
			res := lang.GenerateRules(language.GenerateArgs{
				Config:       c,
				Dir:          dir,
				Rel:          rel,
				File:         oldFile,
				Subdirs:      subdirs,
				RegularFiles: regularFiles,
				GenFiles:     genFiles,
			})
			if len(res.Empty) > 0 {
				t.Errorf("got %d empty rules; want 0", len(res.Empty))
			}
			f := rule.EmptyFile("test", "")
			for _, r := range res.Gen {
				r.Insert(f)
			}
			convertImportsAttrs(f)
			merger.FixLoads(f, lang.(language.ModuleAwareLanguage).ApparentLoads(func(string) string { return "" }))
			f.Sync()
			got := string(bzl.Format(f.File))
			wantPath := filepath.Join(dir, "BUILD.want")
			wantBytes, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("error reading %s: %v", wantPath, err)
			}
			want := string(wantBytes)

			if got != want {
				t.Errorf("GenerateRules %q: got:\n%s\nwant:\n%s", rel, got, want)
			}
		})
	})
}

func TestGenerateRulesEmpty(t *testing.T) {
	lang := NewLanguage()
	c := config.New()
	c.Exts[protoName] = &ProtoConfig{}

	oldContent := []byte(`
proto_library(
    name = "dead_proto",
    srcs = ["foo.proto"],
)

proto_library(
    name = "live_proto",
    srcs = ["bar.proto"],
)

COMPLICATED_SRCS = ["baz.proto"]

proto_library(
    name = "complicated_proto",
    srcs = COMPLICATED_SRCS,
)
`)
	old, err := rule.LoadData("BUILD.bazel", "", oldContent)
	if err != nil {
		t.Fatal(err)
	}
	genFiles := []string{"bar.proto"}
	res := lang.GenerateRules(language.GenerateArgs{
		Config:   c,
		Rel:      "foo",
		File:     old,
		GenFiles: genFiles,
	})
	if len(res.Gen) > 0 {
		t.Errorf("got %d generated rules; want 0", len(res.Gen))
	}
	f := rule.EmptyFile("test", "")
	for _, r := range res.Empty {
		r.Insert(f)
	}
	f.Sync()
	got := strings.TrimSpace(string(bzl.Format(f.File)))
	want := `proto_library(name = "dead_proto")`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestGeneratePackage(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO(jayconrod): set up testdata directory on windows before running test
		if _, err := os.Stat("testdata"); os.IsNotExist(err) {
			t.Skip("testdata missing on windows due to lack of symbolic links")
		} else if err != nil {
			t.Fatal(err)
		}
	}

	lang := NewLanguage()
	c, _, _ := testConfig(t, "testdata")
	dir := filepath.FromSlash("testdata/protos")
	res := lang.GenerateRules(language.GenerateArgs{
		Config:       c,
		Dir:          dir,
		Rel:          "protos",
		RegularFiles: []string{"foo.proto"},
	})
	r := res.Gen[0]
	got := r.PrivateAttr(PackageKey).(Package)
	want := Package{
		Name: "bar.foo",
		Files: map[string]FileInfo{
			"foo.proto": {
				Path:        filepath.Join(dir, "foo.proto"),
				Name:        "foo.proto",
				PackageName: "bar.foo",
				Options:     []Option{{Key: "go_package", Value: "example.com/repo/protos"}},
				Imports: []string{
					"google/protobuf/any.proto",
					"protos/sub/sub.proto",
				},
				HasServices: true,
				Services:    []string{"Quux"},
			},
		},
		Imports: map[string]bool{
			"google/protobuf/any.proto": true,
			"protos/sub/sub.proto":      true,
		},
		Options: map[string]string{
			"go_package": "example.com/repo/protos",
		},
		HasServices: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v; want %#v", got, want)
	}
}

func TestFileModeImports(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO(jayconrod): set up testdata directory on windows before running test
		if _, err := os.Stat("testdata"); os.IsNotExist(err) {
			t.Skip("testdata missing on windows due to lack of symbolic links")
		} else if err != nil {
			t.Fatal(err)
		}
	}

	lang := NewLanguage()
	c, _, _ := testConfig(t, "testdata")
	c.Exts[protoName] = &ProtoConfig{
		Mode: FileMode,
	}

	dir := filepath.FromSlash("testdata/file_mode")
	res := lang.GenerateRules(language.GenerateArgs{
		Config:       c,
		Dir:          dir,
		Rel:          "file_mode",
		RegularFiles: []string{"foo.proto", "bar.proto"},
	})

	if len(res.Gen) != 2 {
		t.Error("expected 2 generated packages")
	}

	bar := res.Gen[0].PrivateAttr(PackageKey).(Package)
	foo := res.Gen[1].PrivateAttr(PackageKey).(Package)

	// I believe the packages are sorted by name, but just in case..
	if bar.RuleName == "foo" {
		bar, foo = foo, bar
	}

	expectedFoo := Package{
		Name:     "file_mode",
		RuleName: "foo",
		Files: map[string]FileInfo{
			"foo.proto": {
				Path:        filepath.Join(dir, "foo.proto"),
				Name:        "foo.proto",
				PackageName: "file_mode",
				Messages:    []string{"Foo"},
				HasMessages: true,
				HasServices: false,
			},
		},
		Imports:     map[string]bool{},
		Options:     map[string]string{},
		HasMessages: true,
		HasServices: false,
	}

	expectedBar := Package{
		Name:     "file_mode",
		RuleName: "bar",
		Files: map[string]FileInfo{
			"bar.proto": {
				Path:        filepath.Join(dir, "bar.proto"),
				Name:        "bar.proto",
				PackageName: "file_mode",
				Imports: []string{
					"file_mode/foo.proto",
				},
				Messages:    []string{"Bar"},
				HasMessages: true,
				HasServices: false,
			},
		},
		// Imports should contain foo.proto. This is specific to file mode.
		// In package mode, this import would be omitted as both foo.proto
		// and bar.proto exist within the same package.
		Imports: map[string]bool{
			"file_mode/foo.proto": true,
		},
		Options:     map[string]string{},
		HasMessages: true,
		HasServices: false,
	}

	if !reflect.DeepEqual(foo, expectedFoo) {
		t.Errorf("got %#v; want %#v", foo, expectedFoo)
	}
	if !reflect.DeepEqual(bar, expectedBar) {
		t.Errorf("got %#v; want %#v", bar, expectedBar)
	}
}

// TestConsumedGenFiles checks that generated files that have been consumed by
// other rules should not be added to the rule
func TestConsumedGenFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO(jayconrod): set up testdata directory on windows before running test
		if _, err := os.Stat("testdata"); os.IsNotExist(err) {
			t.Skip("testdata missing on windows due to lack of symbolic links")
		} else if err != nil {
			t.Fatal(err)
		}
	}

	oldContent := []byte(`
proto_library(
    name = "existing_gen_proto",
    srcs = ["gen.proto"],
)
proto_library(
    name = "dead_proto",
    srcs = ["dead.proto"],
)
`)
	old, err := rule.LoadData("BUILD.bazel", "", oldContent)
	if err != nil {
		t.Fatal(err)
	}

	genRule1 := rule.NewRule("proto_library", "gen_proto")
	genRule1.SetAttr("srcs", []string{"gen.proto"})
	genRule2 := rule.NewRule("filegroup", "filegroup_protos")
	genRule2.SetAttr("srcs", []string{"gen.proto", "gen_not_consumed.proto"})

	c, lang, _ := testConfig(t, "testdata")

	res := lang.GenerateRules(language.GenerateArgs{
		Config:       c,
		Dir:          filepath.FromSlash("testdata/protos"),
		File:         old,
		Rel:          "protos",
		RegularFiles: []string{"foo.proto"},
		GenFiles:     []string{"gen.proto", "gen_not_consumed.proto"},
		OtherGen:     []*rule.Rule{genRule1, genRule2},
	})

	// Make sure that "gen.proto" is not added to existing foo_proto rule
	// because it is consumed by existing_gen_proto proto_library.
	// "gen_not_consumed.proto" is added to existing foo_proto rule because
	// it is not consumed by "proto_library". "filegroup" consumption is
	// ignored.
	fg := rule.EmptyFile("test_gen", "")
	for _, r := range res.Gen {
		r.Insert(fg)
	}
	gotGen := strings.TrimSpace(string(fg.Format()))
	wantGen := `proto_library(
    name = "protos_proto",
    srcs = [
        "foo.proto",
        "gen_not_consumed.proto",
    ],
    visibility = ["//visibility:public"],
)`

	if gotGen != wantGen {
		t.Errorf("got:\n%s\nwant:\n%s", gotGen, wantGen)
	}

	// Make sure that gen.proto is not among empty because it is in GenFiles
	fe := rule.EmptyFile("test_empty", "")
	for _, r := range res.Empty {
		r.Insert(fe)
	}
	got := strings.TrimSpace(string(fe.Format()))
	want := `proto_library(name = "dead_proto")`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func testConfig(t *testing.T, repoRoot string) (*config.Config, language.Language, []config.Configurer) {
	cexts := []config.Configurer{
		&config.CommonConfigurer{},
		&walk.Configurer{},
		&resolve.Configurer{},
	}
	lang := NewLanguage()
	c := testtools.NewTestConfig(t, cexts, []language.Language{lang}, []string{
		"-build_file_name=BUILD.old",
		"-repo_root=" + repoRoot,
	})
	cexts = append(cexts, lang)
	return c, lang, cexts
}

// convertImportsAttrs copies private attributes to regular attributes, which
// will later be written out to build files. This allows tests to check the
// values of private attributes with simple string comparison.
func convertImportsAttrs(f *rule.File) {
	for _, r := range f.Rules {
		v := r.PrivateAttr(config.GazelleImportsKey)
		if v != nil {
			r.SetAttr(config.GazelleImportsKey, v)
		}
	}
}
