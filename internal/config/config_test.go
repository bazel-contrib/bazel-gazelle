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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/internal/rule"
)

func TestPreprocessTags(t *testing.T) {
	c := &Config{
		GenericTags: map[string]bool{"a": true, "b": true},
	}
	c.PreprocessTags()
	expectedTags := []string{"a", "b", "gc"}
	for _, tag := range expectedTags {
		if !c.GenericTags[tag] {
			t.Errorf("tag %q not set", tag)
		}
	}
	unexpectedTags := []string{"x", "cgo", "go1.8", "go1.7"}
	for _, tag := range unexpectedTags {
		if c.GenericTags[tag] {
			t.Errorf("tag %q unexpectedly set", tag)
		}
	}
}

func TestCommonConfigurerFlags(t *testing.T) {
	dir, err := ioutil.TempDir(os.Getenv("TEST_TEMPDIR"), "config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "WORKSPACE"), nil, 0666); err != nil {
		t.Fatal(err)
	}

	c := New()
	cc := &CommonConfigurer{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cc.RegisterFlags(fs, "test", c)
	args := []string{"-repo_root", dir, "-build_file_name", "x,y"}
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if err := cc.CheckFlags(fs, c); err != nil {
		t.Errorf("CheckFlags: %v", err)
	}

	if c.RepoRoot != dir {
		t.Errorf("for RepoRoot, got %#v, want %#v", c.RepoRoot, dir)
	}

	wantBuildFileNames := []string{"x", "y"}
	if !reflect.DeepEqual(c.ValidBuildFileNames, wantBuildFileNames) {
		t.Errorf("for ValidBuildFileNames, got %#v, want %#v", c.ValidBuildFileNames, wantBuildFileNames)
	}
}

func TestCommonConfigurerDirectives(t *testing.T) {
	c := New()
	cc := &CommonConfigurer{}
	buildData := []byte(`# gazelle:build_file_name x,y`)
	f, err := rule.LoadData("test", buildData)
	if err != nil {
		t.Fatal(err)
	}
	cc.Configure(c, "", f)
	want := []string{"x", "y"}
	if !reflect.DeepEqual(c.ValidBuildFileNames, want) {
		t.Errorf("for ValidBuildFileNames, got %#v, want %#v", c.ValidBuildFileNames, want)
	}
}
