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

package config

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/rule"
)

func TestCommonConfigurerFlags(t *testing.T) {
	dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "WORKSPACE"), nil, 0o666); err != nil {
		t.Fatal(err)
	}

	c := New()
	cc := &CommonConfigurer{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cc.RegisterFlags(fs, "test", c)
	args := []string{"-repo_root", dir, "-lang", "go"}
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if err := cc.CheckFlags(fs, c); err != nil {
		t.Errorf("CheckFlags: %v", err)
	}

	if c.RepoRoot != dir {
		t.Errorf("for RepoRoot, got %#v, want %#v", c.RepoRoot, dir)
	}

	wantLangs := []string{"go"}
	if !reflect.DeepEqual(c.Langs, wantLangs) {
		t.Errorf("for Langs, got %#v, want %#v", c.Langs, wantLangs)
	}
}

func TestCommonConfigurerDirectives(t *testing.T) {
	c := New()
	cc := &CommonConfigurer{}
	buildData := []byte(`# gazelle:lang go`)
	f, err := rule.LoadData(filepath.Join("test", "BUILD.bazel"), "", buildData)
	if err != nil {
		t.Fatal(err)
	}
	cc.Configure(c, "", f)

	wantLangs := []string{"go"}
	if !reflect.DeepEqual(c.Langs, wantLangs) {
		t.Errorf("for Langs, got %#v, want %#v", c.Langs, wantLangs)
	}
}

func TestCommonConfigurerRepoName(t *testing.T) {
	cases := []struct {
		desc     string
		fileName string
		content  string
		want     string
	}{
		{
			desc:     "WORKSPACE",
			fileName: "WORKSPACE",
			content:  `workspace(name = "my_workspace")`,
			want:     "my_workspace",
		},
		{
			desc:     "WORKSPACE without name",
			fileName: "WORKSPACE",
			content:  "",
			want:     "",
		},
		{
			desc:     "MODULE.bazel",
			fileName: "MODULE.bazel",
			content:  `module(name = "my_module", version = "0.1.0")`,
			want:     "my_module",
		},
		{
			desc:     "MODULE.bazel without name",
			fileName: "MODULE.bazel",
			content:  "",
			want:     "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "config_repo_name_test")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			// Bazel can create symlinks in test sandboxes; resolve to real path
			dir, err = filepath.EvalSymlinks(dir)
			if err != nil {
				t.Fatal(err)
			}

			// Write the file that should reveal the repo name.
			if err := os.WriteFile(filepath.Join(dir, tc.fileName), []byte(tc.content), 0o666); err != nil {
				t.Fatal(err)
			}

			// Standard CommonConfigurer setup with preset repo root
			// We'd want to use testtools.NewTestConfig() here but it would introduce cyclic dependency
			c := New()
			cc := &CommonConfigurer{}
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			cc.RegisterFlags(fs, "test", c)
			args := []string{"-repo_root", dir}
			if err := fs.Parse(args); err != nil {
				t.Fatal(err)
			}
			if err := cc.CheckFlags(fs, c); err != nil {
				t.Fatalf("CheckFlags: %v", err)
			}
			cc.Configure(c, "", nil)

			if got := c.RepoName; got != tc.want {
				t.Errorf("for RepoName, got %q, want %q", got, tc.want)
			}
		})
	}
}
