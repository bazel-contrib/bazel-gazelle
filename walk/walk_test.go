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

package walk

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/google/go-cmp/cmp"
)

func TestConfigureCallbackOrder(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{{Path: "a/b/"}})
	defer cleanup()

	var configureRels, callbackRels []string
	c, cexts := testConfig(t, dir)
	cexts = append(cexts, &testConfigurer{func(_ *config.Config, rel string, _ *rule.File) {
		configureRels = append(configureRels, rel)
	}})
	Walk(c, cexts, []string{dir}, VisitAllUpdateSubdirsMode, func(_ string, rel string, _ *config.Config, _ bool, _ *rule.File, _, _, _ []string) {
		callbackRels = append(callbackRels, rel)
	})
	configureWant := []string{"", "a", "a/b"}
	if diff := cmp.Diff(configureWant, configureRels); diff != "" {
		t.Errorf("configure order (-want +got):\n%s", diff)
	}
	callbackWant := []string{"a/b", "a", ""}
	if diff := cmp.Diff(callbackWant, callbackRels); diff != "" {
		t.Errorf("callback order (-want +got):\n%s", diff)
	}
}

func TestUpdateDirs(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "update/sub/"},
		{Path: "update/sub/sub/"},
		{
			Path:    "update/ignore/BUILD.bazel",
			Content: "# gazelle:ignore",
		},
		{Path: "update/ignore/sub/"},
		{
			Path:    "update/error/BUILD.bazel",
			Content: "(",
		},
		{Path: "update/error/sub/"},
	})
	defer cleanup()

	type visitSpec struct {
		Rel    string
		Update bool
	}
	for _, tc := range []struct {
		desc string
		rels []string
		mode Mode
		want []visitSpec
	}{
		{
			desc: "visit_all_update_subdirs",
			rels: []string{"update"},
			mode: VisitAllUpdateSubdirsMode,
			want: []visitSpec{
				{"update/error/sub", true},
				{"update/error", false},
				{"update/ignore/sub", true},
				{"update/ignore", false},
				{"update/sub/sub", true},
				{"update/sub", true},
				{"update", true},
				{"", false},
			},
		}, {
			desc: "visit_all_update_dirs",
			rels: []string{"update", "update/ignore/sub"},
			mode: VisitAllUpdateDirsMode,
			want: []visitSpec{
				{"update/error/sub", false},
				{"update/error", false},
				{"update/ignore/sub", true},
				{"update/ignore", false},
				{"update/sub/sub", false},
				{"update/sub", false},
				{"update", true},
				{"", false},
			},
		}, {
			desc: "update_dirs",
			rels: []string{"update", "update/ignore/sub"},
			mode: UpdateDirsMode,
			want: []visitSpec{
				{"update/ignore/sub", true},
				{"update", true},
			},
		}, {
			desc: "update_subdirs",
			rels: []string{"update/ignore", "update/sub"},
			mode: UpdateSubdirsMode,
			want: []visitSpec{
				{"update/ignore/sub", true},
				{"update/ignore", false},
				{"update/sub/sub", true},
				{"update/sub", true},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			c, cexts := testConfig(t, dir)
			dirs := make([]string, len(tc.rels))
			for i, rel := range tc.rels {
				dirs[i] = filepath.Join(dir, filepath.FromSlash(rel))
			}
			var visits []visitSpec
			Walk(c, cexts, dirs, tc.mode, func(_ string, rel string, _ *config.Config, update bool, _ *rule.File, _, _, _ []string) {
				visits = append(visits, visitSpec{rel, update})
			})
			if diff := cmp.Diff(tc.want, visits); diff != "" {
				t.Errorf("Walk visits (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGenMode(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "mode-create/"},
		{Path: "mode-create/a.go"},
		{Path: "mode-create/sub/"},
		{Path: "mode-create/sub/b.go"},
		{Path: "mode-create/sub/sub2/"},
		{Path: "mode-create/sub/sub2/sub3/c.go"},
		{Path: "mode-update/"},
		{
			Path:    "mode-update/BUILD.bazel",
			Content: "# gazelle:generation_mode update_only",
		},
		{Path: "mode-update/a.go"},
		{Path: "mode-update/sub/"},
		{Path: "mode-update/sub/b.go"},
		{Path: "mode-update/sub/sub2/"},
		{Path: "mode-update/sub/sub2/sub3/c.go"},
		{Path: "mode-update/sub/sub3/"},
		{Path: "mode-update/sub/sub3/BUILD.bazel"},
		{Path: "mode-update/sub/sub3/d.go"},
		{Path: "mode-update/sub/sub3/sub4/"},
		{Path: "mode-update/sub/sub3/sub4/e.go"},
	})
	defer cleanup()

	type visitSpec struct {
		subdirs, files []string
	}

	t.Run("generation_mode create vs update", func(t *testing.T) {
		c, cexts := testConfig(t, dir)
		var visits []visitSpec
		Walk(c, cexts, []string{"."}, VisitAllUpdateSubdirsMode, func(_ string, rel string, _ *config.Config, update bool, _ *rule.File, subdirs, regularFiles, _ []string) {
			visits = append(visits, visitSpec{
				subdirs: subdirs,
				files:   regularFiles,
			})
		})

		if len(visits) != 7 {
			t.Error(fmt.Sprintf("Expected 7 visits, got %v", len(visits)))
		}

		if !reflect.DeepEqual(visits[len(visits)-1].subdirs, []string{"mode-create", "mode-update"}) {
			t.Errorf("Last visit should be root dir with 2 subdirs")
		}

		if len(visits[0].subdirs) != 0 || len(visits[0].files) != 1 || visits[0].files[0] != "c.go" {
			t.Errorf("Leaf visit should be only files: %v", visits[0])
		}
		modeUpdateFiles1 := []string{"BUILD.bazel", "d.go", "sub4/e.go"}
		if !reflect.DeepEqual(visits[4].files, modeUpdateFiles1) {
			t.Errorf("update mode should contain files in subdirs. Want %v, got: %v", modeUpdateFiles1, visits[5].files)
		}

		modeUpdateFiles2 := []string{"BUILD.bazel", "a.go", "sub/b.go", "sub/sub2/sub3/c.go"}
		if !reflect.DeepEqual(visits[5].files, modeUpdateFiles2) {
			t.Errorf("update mode should contain files in subdirs. Want %v, got: %v", modeUpdateFiles2, visits[5].files)
		}
	})
}

func TestCustomBuildName(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{
			Path:    "BUILD.bazel",
			Content: "# gazelle:build_file_name BUILD.test",
		}, {
			Path: "BUILD",
		}, {
			Path: "sub/BUILD.test",
		}, {
			Path: "sub/BUILD.bazel",
		},
	})
	defer cleanup()

	c, cexts := testConfig(t, dir)
	var rels []string
	Walk(c, cexts, []string{dir}, VisitAllUpdateSubdirsMode, func(_ string, _ string, _ *config.Config, _ bool, f *rule.File, _, _, _ []string) {
		rel, err := filepath.Rel(c.RepoRoot, f.Path)
		if err != nil {
			t.Error(err)
		} else {
			rels = append(rels, filepath.ToSlash(rel))
		}
	})
	want := []string{
		"sub/BUILD.test",
		"BUILD.bazel",
	}
	if diff := cmp.Diff(want, rels); diff != "" {
		t.Errorf("Walk relative paths (-want +got):\n%s", diff)
	}
}

func TestExcludeFiles(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
# gazelle:exclude **/*.pb.go
# gazelle:exclude *.gen.go
# gazelle:exclude a.go
# gazelle:exclude c/**/b
# gazelle:exclude gen
# gazelle:exclude ign
# gazelle:exclude sub/b.go

gen(
    name = "x",
    out = "gen",
)`,
		},
		{
			Path: ".bazelignore",
			Content: `
dir
dir2/a/b
dir3/

# Globs are not allowed in .bazelignore so this will not be ignored
foo/*

# Random comment followed by a line
a.file

# Paths can have a ./ prefix
./b.file
././blah/../ugly/c.file
`,
		},
		{Path: ".dot"},       // not ignored
		{Path: "_blank"},     // not ignored
		{Path: "a/a.proto"},  // not ignored
		{Path: "a/b.gen.go"}, // not ignored
		{Path: "dir2/a/c"},   // not ignored
		{Path: "foo/a/c"},    // not ignored

		{Path: "a.gen.go"},        // ignored by '*.gen.go'
		{Path: "a.go"},            // ignored by 'a.go'
		{Path: "a.pb.go"},         // ignored by '**/*.pb.go'
		{Path: "a/a.pb.go"},       // ignored by '**/*.pb.go'
		{Path: "a/b/a.pb.go"},     // ignored by '**/*.pb.go'
		{Path: "c/x/b/foo"},       // ignored by 'c/**/b'
		{Path: "c/x/y/b/bar"},     // ignored by 'c/**/b'
		{Path: "c/x/y/b/foo/bar"}, // ignored by 'c/**/b'
		{Path: "ign/bad"},         // ignored by 'ign'
		{Path: "sub/b.go"},        // ignored by 'sub/b.go'
		{Path: "dir/contents"},    // ignored by .bazelignore 'dir'
		{Path: "dir2/a/b"},        // ignored by .bazelignore 'dir2/a/b'
		{Path: "dir3/g/h"},        // ignored by .bazelignore 'dir3/'
		{Path: "a.file"},          // ignored by .bazelignore 'a.file'
		{Path: "b.file"},          // ignored by .bazelignore './b.file'
		{Path: "ugly/c.file"},     // ignored by .bazelignore '././blah/../ugly/c.file'
	})
	defer cleanup()

	c, cexts := testConfig(t, dir)
	var files []string
	Walk(c, cexts, []string{dir}, VisitAllUpdateSubdirsMode, func(_ string, rel string, _ *config.Config, _ bool, _ *rule.File, _, regularFiles, genFiles []string) {
		for _, f := range regularFiles {
			files = append(files, path.Join(rel, f))
		}
		for _, f := range genFiles {
			files = append(files, path.Join(rel, f))
		}
	})
	want := []string{"a/a.proto", "a/b.gen.go", "dir2/a/c", "foo/a/c", ".bazelignore", ".dot", "BUILD.bazel", "_blank"}
	if diff := cmp.Diff(want, files); diff != "" {
		t.Errorf("Walk files (-want +got):\n%s", diff)
	}
}

func TestExcludeSelf(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
		}, {
			Path:    "sub/BUILD.bazel",
			Content: "# gazelle:exclude .",
		}, {
			Path: "sub/below/BUILD.bazel",
		},
	})
	defer cleanup()

	c, cexts := testConfig(t, dir)
	var rels []string
	Walk(c, cexts, []string{dir}, VisitAllUpdateDirsMode, func(_ string, rel string, _ *config.Config, _ bool, f *rule.File, _, _, _ []string) {
		rels = append(rels, rel)
	})

	want := []string{""}
	if diff := cmp.Diff(want, rels); diff != "" {
		t.Errorf("Walk relative paths (-want +got):\n%s", diff)
	}
}

func TestGeneratedFiles(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
unknown_rule(
    name = "blah1",
    out = "gen1",
)

unknown_rule(
    name = "blah2",
    outs = [
        "gen2",
        "gen-and-static",
    ],
)
`,
		},
		{Path: "gen-and-static"},
		{Path: "static"},
	})
	defer cleanup()

	c, cexts := testConfig(t, dir)
	var regularFiles, genFiles []string
	Walk(c, cexts, []string{dir}, VisitAllUpdateSubdirsMode, func(_ string, rel string, _ *config.Config, _ bool, _ *rule.File, _, reg, gen []string) {
		for _, f := range reg {
			regularFiles = append(regularFiles, path.Join(rel, f))
		}
		for _, f := range gen {
			genFiles = append(genFiles, path.Join(rel, f))
		}
	})
	regWant := []string{"BUILD.bazel", "gen-and-static", "static"}
	if diff := cmp.Diff(regWant, regularFiles); diff != "" {
		t.Errorf("Walk regularFiles (-want +got):\n%s", diff)
	}
	genWant := []string{"gen1", "gen2", "gen-and-static"}
	if diff := cmp.Diff(genWant, genFiles); diff != "" {
		t.Errorf("Walk genFiles (-want +got):\n%s", diff)
	}
}

func testConfig(t *testing.T, dir string) (*config.Config, []config.Configurer) {
	args := []string{"-repo_root", dir}
	cexts := []config.Configurer{&config.CommonConfigurer{}, &Configurer{}}
	c := testtools.NewTestConfig(t, cexts, nil, args)
	return c, cexts
}

var _ config.Configurer = (*testConfigurer)(nil)

type testConfigurer struct {
	configure func(c *config.Config, rel string, f *rule.File)
}

func (*testConfigurer) RegisterFlags(_ *flag.FlagSet, _ string, _ *config.Config) {}

func (*testConfigurer) CheckFlags(_ *flag.FlagSet, _ *config.Config) error { return nil }

func (*testConfigurer) KnownDirectives() []string { return nil }

func (tc *testConfigurer) Configure(c *config.Config, rel string, f *rule.File) {
	tc.configure(c, rel, f)
}

// BenchmarkWalk measures how long it takes Walk to traverse a synthetic repo.
//
// There are 10 top-level directories. Each has 10 subdirectories. Each of
// those has 10 subdirectories (so 1001 directories in total).
//
// Each directory has 10 files and a BUILD file with a filegroup that includes
// those files (the content isn't really important, we just want to exercise
// the parser a little bit.)
//
// This is somewhat unrealistic: the whole tree is likely to be in the kernel's
// memory in the kernel's file cache, so this doesn't measure I/O to disk.
// Still, this is frequently true for real projects where Gazelle is invoked.
func BenchmarkWalk(b *testing.B) {
	// Create a fake repo to walk.
	subdirCount := 10
	fileCount := 10
	levelCount := 3

	buildFileBuilder := &bytes.Buffer{}
	fmt.Fprintf(buildFileBuilder, "filegroup(\n    srcs = [\n")
	for i := range fileCount {
		fmt.Fprintf(buildFileBuilder, "        \"f%d\",\n", i)
	}
	fmt.Fprintf(buildFileBuilder, "    ],\n)\n")
	buildFileContent := buildFileBuilder.Bytes()

	rootDir := b.TempDir()
	var createDir func(string, int)
	createDir = func(dir string, level int) {
		buildFilePath := filepath.Join(dir, "BUILD")
		if err := os.WriteFile(buildFilePath, buildFileContent, 0666); err != nil {
			b.Fatal(err)
		}

		for i := range fileCount {
			filePath := filepath.Join(dir, fmt.Sprintf("f%d", i))
			if err := os.WriteFile(filePath, nil, 0666); err != nil {
				b.Fatal(err)
			}
		}

		if level < levelCount {
			for i := range subdirCount {
				subdir := filepath.Join(dir, fmt.Sprintf("d%d", i))
				if err := os.Mkdir(subdir, 0777); err != nil {
					b.Fatal(err)
				}
				createDir(subdir, level+1)
			}
		}
	}
	createDir(rootDir, 0)

	cexts := []config.Configurer{&Configurer{}}
	c := config.New()
	c.RepoRoot = rootDir
	c.RepoRoot = rootDir
	c.IndexLibraries = true
	fs := flag.NewFlagSet("gazelle", flag.ContinueOnError)
	for _, cext := range cexts {
		cext.RegisterFlags(fs, "update", c)
	}

	// Benchmark calling Walk with a trivial callback function.
	wf := func(dir, rel string, c *config.Config, update bool, f *rule.File, subdirs, regularFiles, genFiles []string) {
	}

	b.ResetTimer()
	for range b.N {
		Walk(c, nil, []string{rootDir}, VisitAllUpdateSubdirsMode, wf)
	}
}
