/*
	Copyright 2016 The Bazel Authors. All rights reserved.

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
package wspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFind(t *testing.T) {
	tmp, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	tmp, err = filepath.EvalSymlinks(tmp) // on macOS, TEST_TEMPDIR is a symlink
	if err != nil {
		t.Fatal(err)
	}
	if parent, err := FindRepoRoot(tmp); err == nil {
		t.Skipf("WORKSPACE visible in parent %q of tmp %q", parent, tmp)
	}

	for _, tc := range []struct {
		file, testdir string // file == "" ==> do not create file
		shouldSucceed bool
	}{
		{"", tmp, false},
		{filepath.Join(tmp, "WORKSPACE"), tmp, true},
		{filepath.Join(tmp, "WORKSPACE.bazel"), tmp, true},
		{filepath.Join(tmp, "WORKSPACE.bazel"), filepath.Join(tmp, "dir1"), true},
		// Test within a directory name WORKSPACE
		{filepath.Join(tmp, "WORKSPACE"), filepath.Join(tmp, "dir1", "WORKSPACE", "dir2"), true},
		{filepath.Join(tmp, "WORKSPACE.bazel"), filepath.Join(tmp, "dir1", "WORKSPACE", "dir2"), true},
		// Test outside a workspace but within a directory named WORKSPACE
		{filepath.Join(tmp, "WORKSPACE", "file.txt"), filepath.Join(tmp, "dir1"), false},
		{filepath.Join(tmp, "WORKSPACE", "file.txt"), filepath.Join(tmp, "WORKSPACE", "dir1"), false},
	} {
		t.Run(tc.file, func(t *testing.T) {
			if err := os.RemoveAll(tmp); err != nil {
				t.Fatal(err)
			}

			if tc.file != "" {
				// Create a WORKSPACE file
				if err := os.MkdirAll(filepath.Dir(tc.file), 0o755); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(tc.file, nil, 0o755); err != nil {
					t.Fatal(err)
				}
			}

			// Create the testdir dir
			if err := os.MkdirAll(tc.testdir, 0o755); err != nil {
				t.Fatal(err)
			}

			// Look for the file
			dir, err := FindRepoRoot(tc.testdir)

			if !tc.shouldSucceed {
				if err == nil {
					t.Errorf("FindRoot(%q): got %v, wanted failure", tc.testdir, dir)
				}
				return
			}

			if err != nil {
				t.Errorf("FindRoot(%q): got error %v, wanted %v", tc.testdir, err, tc.file)
			}

			file := FindWORKSPACEFile(dir)
			if file != tc.file {
				t.Errorf("FindWorkspaceFile(FindRoot(%q)): got %v, wanted %v", tc.testdir, file, tc.file)
			}
		})
	}
}
