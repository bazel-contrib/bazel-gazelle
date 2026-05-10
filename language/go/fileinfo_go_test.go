/* Copyright 2017 The Bazel Authors. All rights reserved.

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	fileInfoCmpOption = cmp.AllowUnexported(
		fileInfo{},
		fileEmbed{},
		buildTags{},
		cgoTagsAndOpts{},
	)
)

func TestGoFileInfo(t *testing.T) {
	for _, tc := range []struct {
		desc, name, source string
		want               fileInfo
	}{
		{
			"empty file",
			"foo.go",
			"package foo\n",
			fileInfo{
				packageName: "foo",
			},
		},
		{
			"xtest file",
			"foo_test.go",
			"package foo_test\n",
			fileInfo{
				packageName: "foo",
				isTest:      true,
			},
		},
		{
			"xtest suffix on non-test",
			"foo_xtest.go",
			"package foo_test\n",
			fileInfo{
				packageName: "foo_test",
				isTest:      false,
			},
		},
		{
			"hash bench test",
			"foo_test.go",
			`package foo_test

func BenchmarkFoo(b *testing.B) {}`,
			fileInfo{
				packageName:       "foo",
				isTest:            true,
				hasBenchmarkTests: true,
			},
		},
		{
			"invalid bench test 1",
			"foo_test.go",
			`package foo_test

func BenchmarkFoo(b *testing.B, a string) {}`,
			fileInfo{
				packageName:       "foo",
				isTest:            true,
				hasBenchmarkTests: false,
			},
		},
		{
			"invalid bench test 2",
			"foo_test.go",
			`package foo_test

func CenchmarkFoo(b *testing.B) {}`,
			fileInfo{
				packageName:       "foo",
				isTest:            true,
				hasBenchmarkTests: false,
			},
		},
		{
			"invalid bench test 3",
			"foo_test.go",
			`package foo_test

func BenchmarkFoo(b testing.B) {}`,
			fileInfo{
				packageName:       "foo",
				isTest:            true,
				hasBenchmarkTests: false,
			},
		},
		{
			"invalid bench test 4",
			"foo_test.go",
			`package foo_test

func BenchmarkFoo(c *testing.C) {}`,
			fileInfo{
				packageName:       "foo",
				isTest:            true,
				hasBenchmarkTests: false,
			},
		},
		{
			"single import",
			"foo.go",
			`package foo

import "github.com/foo/bar"
`,
			fileInfo{
				packageName: "foo",
				imports:     []string{"github.com/foo/bar"},
			},
		},
		{
			"multiple imports",
			"foo.go",
			`package foo

import (
	"github.com/foo/bar"
	x "github.com/local/project/y"
)
`,
			fileInfo{
				packageName: "foo",
				imports:     []string{"github.com/foo/bar", "github.com/local/project/y"},
			},
		},
		{
			"standard imports included",
			"foo.go",
			`package foo

import "fmt"
`,
			fileInfo{
				packageName: "foo",
				imports:     []string{"fmt"},
			},
		},
		{
			"cgo",
			"foo.go",
			`package foo

import "C"
`,
			fileInfo{
				packageName: "foo",
				isCgo:       true,
			},
		},
		{
			"build tags",
			"foo.go",
			`// +build linux darwin

// +build !ignore

package foo
`,
			fileInfo{
				packageName: "foo",
				tags: &buildTags{
					expr:    mustParseBuildTag(t, "(linux || darwin) && !ignore"),
					rawTags: []string{"linux", "darwin", "ignore"},
				},
			},
		},
		{
			"build tags without blank line",
			"route.go",
			`// Copyright 2017

// +build darwin dragonfly freebsd netbsd openbsd

// Package route provides basic functions for the manipulation of
// packet routing facilities on BSD variants.
package route
`,
			fileInfo{
				packageName: "route",
				tags: &buildTags{
					expr:    mustParseBuildTag(t, "darwin || dragonfly || freebsd || netbsd || openbsd"),
					rawTags: []string{"darwin", "dragonfly", "freebsd", "netbsd", "openbsd"},
				},
			},
		},
		{
			"embed",
			"embed.go",
			`package foo

import _ "embed"

//go:embed embed.go
var src string
`,
			fileInfo{
				packageName: "foo",
				imports:     []string{"embed"},
				embeds:      []fileEmbed{{path: "embed.go"}},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "TestGoFileInfo")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			path := filepath.Join(dir, tc.name)
			if err := os.WriteFile(path, []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}

			got := goFileInfo(path, "")
			// Clear fields we don't care about for testing.
			got = fileInfo{
				packageName:       got.packageName,
				isTest:            got.isTest,
				imports:           got.imports,
				embeds:            got.embeds,
				isCgo:             got.isCgo,
				tags:              got.tags,
				hasBenchmarkTests: got.hasBenchmarkTests,
			}
			for i := range got.embeds {
				got.embeds[i] = fileEmbed{path: got.embeds[i].path}
			}

			if diff := cmp.Diff(tc.want, got, fileInfoCmpOption); diff != "" {
				t.Errorf("(-want, +got): %s", diff)
			}

		})
	}
}

func TestGoFileInfoFailure(t *testing.T) {
	dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "TestGoFileInfoFailure")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	name := "foo_linux_amd64.go"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("pakcage foo"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := goFileInfo(path, "")
	want := fileInfo{
		path:   path,
		name:   name,
		ext:    goExt,
		goos:   "linux",
		goarch: "amd64",
	}
	if diff := cmp.Diff(want, got, fileInfoCmpOption); diff != "" {
		t.Errorf("(-want, +got): %s", diff)
	}

}

func TestCgo(t *testing.T) {
	for _, tc := range []struct {
		desc, source string
		want         fileInfo
	}{
		{
			"not cgo",
			"package foo\n",
			fileInfo{isCgo: false},
		},
		{
			"empty cgo",
			`package foo

import "C"
`,
			fileInfo{isCgo: true},
		},
		{
			"simple flags",
			`package foo

/*
#cgo CFLAGS: -O0
	#cgo CPPFLAGS: -O1
#cgo   CXXFLAGS:   -O2
#cgo LDFLAGS: -O3 -O4
*/
import "C"
`,
			fileInfo{
				isCgo: true,
				cppopts: []*cgoTagsAndOpts{
					{opts: "-O1"},
				},
				copts: []*cgoTagsAndOpts{
					{opts: "-O0"},
				},
				cxxopts: []*cgoTagsAndOpts{
					{opts: "-O2"},
				},
				clinkopts: []*cgoTagsAndOpts{
					{opts: strings.Join([]string{"-O3", "-O4"}, optSeparator)},
				},
			},
		},
		{
			"cflags with conditions",
			`package foo

/*
#cgo foo bar,!baz CFLAGS: -O0
*/
import "C"
`,
			fileInfo{
				isCgo: true,
				copts: []*cgoTagsAndOpts{
					{
						buildTags: &buildTags{
							expr:    mustParseBuildTag(t, "foo || (bar && !baz)"),
							rawTags: []string{"foo", "bar", "baz"},
						},
						opts: "-O0",
					},
				},
			},
		},
		{
			"slashslash comments",
			`package foo

// #cgo CFLAGS: -O0
// #cgo CFLAGS: -O1
import "C"
`,
			fileInfo{
				isCgo: true,
				copts: []*cgoTagsAndOpts{
					{opts: "-O0"},
					{opts: "-O1"},
				},
			},
		},
		{
			"comment above single import group",
			`package foo

/*
#cgo CFLAGS: -O0
*/
import ("C")
`,
			fileInfo{
				isCgo: true,
				copts: []*cgoTagsAndOpts{
					{opts: "-O0"},
				},
			},
		},
		{
			"noescape and nocallback directives",
			`package foo

/*
#cgo noescape my_func
#cgo nocallback my_func
void my_func(void *param);

// This directive won't be parsed if we failed
// to parse the previous directives.
#cgo CFLAGS: -O0
*/
import "C"
`,
			fileInfo{
				isCgo: true,
				copts: []*cgoTagsAndOpts{
					{opts: "-O0"},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "TestCgo")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			name := "TestCgo.go"
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}

			got := goFileInfo(path, "")

			// Clear fields we don't care about for testing.
			got = fileInfo{
				isCgo:     got.isCgo,
				copts:     got.copts,
				cppopts:   got.cppopts,
				cxxopts:   got.cxxopts,
				clinkopts: got.clinkopts,
			}

			if diff := cmp.Diff(tc.want, got, fileInfoCmpOption); diff != "" {
				t.Errorf("(-want, +got): %s", diff)
			}

		})
	}
}

// Copied from go/build build_test.go
var (
	expandSrcDirPath = filepath.Join(string(filepath.Separator)+"projects", "src", "add")
)

// Copied from go/build build_test.go
var expandSrcDirTests = []struct {
	input, expected string
}{
	{"-L ${SRCDIR}/libs -ladd", "-L /projects/src/add/libs -ladd"},
	{"${SRCDIR}/add_linux_386.a -pthread -lstdc++", "/projects/src/add/add_linux_386.a -pthread -lstdc++"},
	{"Nothing to expand here!", "Nothing to expand here!"},
	{"$", "$"},
	{"$$", "$$"},
	{"${", "${"},
	{"$}", "$}"},
	{"$FOO ${BAR}", "$FOO ${BAR}"},
	{"Find me the $SRCDIRECTORY.", "Find me the $SRCDIRECTORY."},
	{"$SRCDIR is missing braces", "$SRCDIR is missing braces"},
}

// Copied from go/build build_test.go
func TestExpandSrcDir(t *testing.T) {
	for _, test := range expandSrcDirTests {
		output, _ := expandSrcDir(test.input, expandSrcDirPath)
		if output != test.expected {
			t.Errorf("%q expands to %q with SRCDIR=%q when %q is expected", test.input, output, expandSrcDirPath, test.expected)
		} else {
			t.Logf("%q expands to %q with SRCDIR=%q", test.input, output, expandSrcDirPath)
		}
	}
}

var (
	goPackageCmpOption = cmp.AllowUnexported(
		goPackage{},
		goTarget{},
		protoTarget{},
		platformStringsBuilder{},
		platformStringInfo{},
	)
)

func TestExpandSrcDirRepoRelative(t *testing.T) {
	repo, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "repo")
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(sub, "sub.go")
	content := []byte(`package sub

/*
#cgo CFLAGS: -I${SRCDIR}/..
*/
import "C"
`)
	if err := os.WriteFile(goFile, content, 0o644); err != nil {
		t.Fatal(err)
	}
	c, _, _ := testConfig(
		t,
		"-repo_root="+repo,
		"-go_prefix=example.com/repo")
	fi := goFileInfo(filepath.Join(sub, "sub.go"), "sub")
	pkgs, _ := buildPackages(c, sub, "sub", false, nil, []fileInfo{fi})
	got, ok := pkgs["sub"]
	if !ok {
		t.Fatal("did not build package 'sub'")
	}
	want := &goPackage{
		name:    "sub",
		dir:     sub,
		rel:     "sub",
		library: goTarget{cgo: true},
	}
	want.library.sources.addGenericString("sub.go")
	want.library.copts.addGenericString("-Isub/..")
	if diff := cmp.Diff(want, got, goPackageCmpOption); diff != "" {
		t.Errorf("(-want, +got): %s", diff)
	}

}

// Copied from go/build build_test.go
func TestShellSafety(t *testing.T) {
	tests := []struct {
		input, srcdir, expected string
		result                  bool
	}{
		{"-I${SRCDIR}/../include", "/projects/src/issue 11868", "-I/projects/src/issue 11868/../include", true},
		{"-I${SRCDIR}", "wtf$@%", "-Iwtf$@%", true},
		{"-X${SRCDIR}/1,${SRCDIR}/2", "/projects/src/issue 11868", "-X/projects/src/issue 11868/1,/projects/src/issue 11868/2", true},
		{"-I/tmp -I/tmp", "/tmp2", "-I/tmp -I/tmp", false},
		{"-I/tmp", "/tmp/[0]", "-I/tmp", true},
		{"-I${SRCDIR}/dir", "/tmp/[0]", "-I/tmp/[0]/dir", false},
	}
	for _, test := range tests {
		output, ok := expandSrcDir(test.input, test.srcdir)
		if ok != test.result {
			t.Errorf("Expected %t while %q expands to %q with SRCDIR=%q; got %t", test.result, test.input, output, test.srcdir, ok)
		}
		if output != test.expected {
			t.Errorf("Expected %q while %q expands with SRCDIR=%q; got %q", test.expected, test.input, test.srcdir, output)
		}
	}
}

func TestGoExperimentsTagsIgnored(t *testing.T) {
	dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "TestGoExperimentsTagsIgnored")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	for _, tc := range []struct {
		desc, content string
	}{
		{
			desc:    "goexperiment tag satisfied",
			content: "//go:build goexperiment.range\n\npackage foo",
		},
		{
			desc:    "negated goexperiments tag satisfied",
			content: "//go:build !goexperiment.range\n\npackage foo",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			path := filepath.Join(dir, tc.desc+".go")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}

			c, _, _ := testConfig(t)
			fi := goFileInfo(path, "")
			if !checkConstraints(c, "", "", fi.goos, fi.goarch, fi.tags, nil) {
				t.Fatalf("constraints should be satisfied for %s", tc.desc)
			}
		})
	}
}

func mustParseBuildTag(t *testing.T, in string) constraint.Expr {
	x, err := constraint.Parse("//go:build " + in)
	if err != nil {
		t.Fatalf("%s: %s", in, err)
	}

	return x
}
