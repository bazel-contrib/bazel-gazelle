/* Copyright 2020 The Bazel Authors. All rights reserved.

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

package bazel_test

import (
	"os"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestRunner(t *testing.T) {
	origBuildData, err := os.ReadFile("BUILD.bazel")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile("BUILD.bazel", origBuildData, 0o666); err != nil {
			t.Fatalf("restoring build file: %v", err)
		}
	}()

	if err := bazel_testing.RunBazel("run", "//:gazelle"); err != nil {
		t.Fatal(err)
	}
	out, err := bazel_testing.BazelOutput("query", "//:all")
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool)
	for _, target := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		got[target] = true
	}
	want := []string{"//:m", "//:m_lib"}
	for _, target := range want {
		if !got[target] {
			t.Errorf("target missing from query output: %s", target)
		}
	}
}

func TestRunnerUpdateReposFromGoMod(t *testing.T) {
	origWorkspaceData, err := os.ReadFile("WORKSPACE")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile("WORKSPACE", origWorkspaceData, 0o666); err != nil {
			t.Fatalf("restoring WORKSPACE: %v", err)
		}
	}()

	if err := bazel_testing.RunBazel("run", "//:gazelle", "--", "update-repos", "-from_file=go.mod"); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerUpdateReposCommand(t *testing.T) {
	origWorkspaceData, err := os.ReadFile("WORKSPACE")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile("WORKSPACE", origWorkspaceData, 0o666); err != nil {
			t.Fatalf("restoring WORKSPACE: %v", err)
		}
	}()

	if err := bazel_testing.RunBazel("run", "//:gazelle-update-repos"); err != nil {
		t.Fatal(err)
	}
}
