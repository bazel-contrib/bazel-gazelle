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

package testtools

import (
	"testing"

	v2 "github.com/bazel-contrib/bazel-gazelle/v2/testtools"
)

// FileSpec specifies the content of a test file.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/testtools.FileSpec instead.
//go:fix inline
type FileSpec = v2.FileSpec

// CreateFiles creates a directory of test files. This is a more compact
// alternative to testdata directories. CreateFiles returns a canonical path
// to the directory and a function to call to clean up the directory
// after the test.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/testtools.CreateFiles instead.
//go:fix inline
func CreateFiles(t *testing.T, files []FileSpec) (dir string, cleanup func()) {
	return v2.CreateFiles(t, files)
}

// CheckFiles checks that files in "dir" exist and have the content specified
// in "files". Files not listed in "files" are not tested, so extra files
// are allowed.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/testtools.CheckFiles instead.
//go:fix inline
func CheckFiles(t *testing.T, dir string, files []FileSpec) {
	v2.CheckFiles(t, dir, files)
}

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/testtools.TestGazelleGenerationArgs instead.
//go:fix inline
type TestGazelleGenerationArgs = v2.TestGazelleGenerationArgs

// TestGazelleGenerationOnPath runs a full gazelle binary on a testdata directory.
// With a test data directory of the form:
// └── <testDataPath>
//
//	└── some_test
//	    ├── WORKSPACE
//	    ├── README.md --> README describing what the test does.
//	    ├── arguments.txt --> newline delimited list of arguments to pass in (ignored if empty).
//	    ├── expectedStdout.txt --> Expected stdout for this test.
//	    ├── expectedStderr.txt --> Expected stderr for this test.
//	    ├── expectedExitCode.txt --> Expected exit code for this test.
//	    └── app
//	        └── sourceFile.foo
//	        └── BUILD.in --> BUILD file prior to running gazelle.
//	        └── BUILD.out --> BUILD file expected after running gazelle.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/testtools.TestGazelleGenerationOnPath instead.
//go:fix inline
func TestGazelleGenerationOnPath(t *testing.T, args *TestGazelleGenerationArgs) {
	v2.TestGazelleGenerationOnPath(t, args)
}
