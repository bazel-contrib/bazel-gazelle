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

package generationtest

import (
	"flag"
	"path/filepath"
	"testing"
	"time"

	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

var (
	gazelleBinaryPath = flag.String("gazelle_binary_path", "", "Path to the gazelle binary to test.")
	buildInSuffix     = flag.String("build_in_suffix", ".in", "The suffix on the test input BUILD.bazel files. Defaults to .in. "+
		" By default, will use files named BUILD.in as the BUILD files before running gazelle.")
	buildOutSuffix = flag.String("build_out_suffix", ".out", "The suffix on the expected BUILD.bazel files after running gazelle. Defaults to .out. "+
		" By default, will use files named BUILD.out as the expected results of the gazelle run.")
	timeout = flag.Duration("timeout", 2*time.Second, "Time to allow the gazelle process to run before killing.")
)

// TestFullGeneration runs the gazelle binary on a few example
// workspaces and confirms that the generated BUILD files match expectation.
func TestFullGeneration(t *testing.T) {
	tests := []*testtools.TestGazelleGenerationArgs{}
	runfiles, err := bazel.ListRunfiles()
	if err != nil {
		t.Fatalf("bazel.ListRunfiles() error: %v", err)
	}
	// Convert workspace relative path for gazelle binary into an absolute path.
	// E.g. path/to/gazelle_binary -> /absolute/path/to/workspace/path/to/gazelle/binary.
	absoluteGazelleBinary, err := bazel.Runfile(*gazelleBinaryPath)
	if err != nil {
		t.Fatalf("Could not convert gazelle binary path %s to absolute path. Error: %v", *gazelleBinaryPath, err)
	}
	testNames := map[string]struct{}{}
	for _, f := range runfiles {
		// Look through runfiles for WORKSPACE or MODULE.bazel files. Each such file specifies a test case.
		if filepath.Base(f.Path) == "WORKSPACE" || filepath.Base(f.Path) == "MODULE.bazel" {
			// absolutePathToTestDirectory is the absolute
			// path to the test case directory. For example, /home/<user>/wksp/path/to/test_data/my_test_case
			absolutePathToTestDirectory := filepath.Dir(f.Path)
			// relativePathToTestDirectory is the workspace relative path
			// to this test case directory. For example, path/to/test_data/my_test_case
			relativePathToTestDirectory := filepath.Dir(f.ShortPath)
			// name is the name of the test directory. For example, my_test_case.
			// The name of the directory doubles as the name of the test.
			name := filepath.Base(absolutePathToTestDirectory)

			// Don't add a test if it was already added. That could be the case if a directory has
			// both a WORKSPACE and a MODULE.bazel file in it.
			if _, exists := testNames[name]; exists {
				continue
			}
			testNames[name] = struct{}{}

			tests = append(tests, &testtools.TestGazelleGenerationArgs{
				Name:                 name,
				TestDataPathAbsolute: absolutePathToTestDirectory,
				TestDataPathRelative: relativePathToTestDirectory,
				GazelleBinaryPath:    absoluteGazelleBinary,
				BuildInSuffix:        *buildInSuffix,
				BuildOutSuffix:       *buildOutSuffix,
				Timeout:              *timeout,
			})
		}
	}
	if len(tests) == 0 {
		t.Fatal("no tests found")
	}

	for _, args := range tests {
		testtools.TestGazelleGenerationOnPath(t, args)
	}
}
