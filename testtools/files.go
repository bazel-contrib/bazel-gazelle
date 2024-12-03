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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const cmdTimeoutOrInterruptExitCode = -1

// FileSpec specifies the content of a test file.
type FileSpec struct {
	// Path is a slash-separated path relative to the test directory. If Path
	// ends with a slash, it indicates a directory should be created
	// instead of a file.
	Path string

	// Symlink is a slash-separated path relative to the test directory. If set,
	// it indicates a symbolic link should be created with this path instead of a
	// file.
	Symlink string

	// Content is the content of the test file.
	Content string

	// NotExist asserts that no file at this path exists.
	// It is only valid in CheckFiles.
	NotExist bool
}

// CreateFiles creates a directory of test files. This is a more compact
// alternative to testdata directories. CreateFiles returns a canonical path
// to the directory and a function to call to clean up the directory
// after the test.
func CreateFiles(t *testing.T, files []FileSpec) (dir string, cleanup func()) {
	t.Helper()
	dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "gazelle_test")
	if err != nil {
		t.Fatal(err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if f.NotExist {
			t.Fatalf("CreateFiles: NotExist may not be set: %s", f.Path)
		}
		path := filepath.Join(dir, filepath.FromSlash(f.Path))
		if strings.HasSuffix(f.Path, "/") {
			if err := os.MkdirAll(path, 0o700); err != nil {
				os.RemoveAll(dir)
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			os.RemoveAll(dir)
			t.Fatal(err)
		}
		if f.Symlink != "" {
			if err := os.Symlink(f.Symlink, path); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(f.Content), 0o600); err != nil {
			os.RemoveAll(dir)
			t.Fatal(err)
		}
	}

	return dir, func() { os.RemoveAll(dir) }
}

// CheckFiles checks that files in "dir" exist and have the content specified
// in "files". Files not listed in "files" are not tested, so extra files
// are allowed.
func CheckFiles(t *testing.T, dir string, files []FileSpec) {
	t.Helper()
	for _, f := range files {
		path := filepath.Join(dir, f.Path)

		st, err := os.Stat(path)
		if f.NotExist {
			if err == nil {
				t.Errorf("asserted to not exist, but does: %s", f.Path)
			} else if !os.IsNotExist(err) {
				t.Errorf("could not stat %s: %v", f.Path, err)
			}
			continue
		}

		if strings.HasSuffix(f.Path, "/") {
			if err != nil {
				t.Errorf("could not stat %s: %v", f.Path, err)
			} else if !st.IsDir() {
				t.Errorf("not a directory: %s", f.Path)
			}
		} else {
			want := normalizeSpace(f.Content)
			gotBytes, err := os.ReadFile(filepath.Join(dir, f.Path))
			if err != nil {
				t.Errorf("could not read %s: %v", f.Path, err)
				continue
			}
			got := normalizeSpace(string(gotBytes))
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("%s diff (-want,+got):\n%s", f.Path, diff)
			}
		}
	}
}

type TestGazelleGenerationArgs struct {
	// Name is the name of the test.
	Name string
	// TestDataPathAbsolute is the absolute path to the test data directory.
	// For example, /home/user/workspace/path/to/test_data/my_testcase.
	TestDataPathAbsolute string
	// TestDataPathRealtive is the workspace relative path to the test data directory.
	// For example, path/to/test_data/my_testcase.
	TestDataPathRelative string
	// GazelleBinaryPath is the workspace relative path to the location of the gazelle binary
	// we want to test.
	GazelleBinaryPath string

	// BuildInSuffix is the suffix for all test input build files. Includes the ".".
	// Default: ".in", so input BUILD files should be named BUILD.in.
	BuildInSuffix string

	// BuildOutSuffix is the suffix for all test output build files. Includes the ".".
	// Default: ".out", so out BUILD files should be named BUILD.out.
	BuildOutSuffix string

	// Timeout is the duration after which the generation process will be killed.
	Timeout time.Duration
}

var (
	argumentsFilename        = "arguments.txt"
	expectedStdoutFilename   = "expectedStdout.txt"
	expectedStderrFilename   = "expectedStderr.txt"
	expectedExitCodeFilename = "expectedExitCode.txt"
)

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
func TestGazelleGenerationOnPath(t *testing.T, args *TestGazelleGenerationArgs) {
	t.Run(args.Name, func(t *testing.T) {
		t.Helper() // Make the stack trace a little bit more clear.
		if args.BuildInSuffix == "" {
			args.BuildInSuffix = ".in"
		}
		if args.BuildOutSuffix == "" {
			args.BuildOutSuffix = ".out"
		}
		var inputs []FileSpec
		var goldens []FileSpec

		config := &testConfig{}
		f := func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				t.Fatalf("File walk error on path %q. Error: %v", path, err)
			}

			shortPath := strings.TrimPrefix(path, args.TestDataPathAbsolute)

			info, err := d.Info()
			if err != nil {
				t.Fatalf("File info error on path %q. Error: %v", path, err)
			}

			if info.IsDir() {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("os.ReadFile(%q) error: %v", path, err)
			}

			// Read in expected stdout, stderr, and exit code files.
			if d.Name() == argumentsFilename {
				config.Args = strings.Split(normalizeSpace(string(content)), "\n")
				return nil
			}
			if d.Name() == expectedStdoutFilename {
				config.Stdout = string(content)
				return nil
			}
			if d.Name() == expectedStderrFilename {
				config.Stderr = string(content)
				return nil
			}
			if d.Name() == expectedExitCodeFilename {
				config.ExitCode, err = strconv.Atoi(strings.TrimSpace(string(content)))
				if err != nil {
					// Set the ExitCode to a sentinel value (-1) to ensure that if the caller is updating the files on disk the value is updated.
					config.ExitCode = -1
					t.Errorf("Failed to parse expected exit code (%q) error: %v", path, err)
				}
				return nil
			}

			if strings.HasSuffix(shortPath, args.BuildInSuffix) {
				inputs = append(inputs, FileSpec{
					Path:    filepath.Join(args.Name, strings.TrimSuffix(shortPath, args.BuildInSuffix)+".bazel"),
					Content: string(content),
				})
			} else if strings.HasSuffix(shortPath, args.BuildOutSuffix) {
				goldens = append(goldens, FileSpec{
					Path:    filepath.Join(args.Name, strings.TrimSuffix(shortPath, args.BuildOutSuffix)+".bazel"),
					Content: string(content),
				})
			} else {
				inputs = append(inputs, FileSpec{
					Path:    filepath.Join(args.Name, shortPath),
					Content: string(content),
				})
				goldens = append(goldens, FileSpec{
					Path:    filepath.Join(args.Name, shortPath),
					Content: string(content),
				})
			}
			return nil
		}
		if err := filepath.WalkDir(args.TestDataPathAbsolute, f); err != nil {
			t.Fatal(err)
		}

		testdataDir, cleanup := CreateFiles(t, inputs)
		workspaceRoot := filepath.Join(testdataDir, args.Name)

		var stdout, stderr bytes.Buffer
		var actualExitCode int
		defer cleanup()
		defer func() {
			if t.Failed() {
				shouldUpdate := os.Getenv("UPDATE_SNAPSHOTS") != ""
				buildWorkspaceDirectory := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
				updateCommand := fmt.Sprintf("UPDATE_SNAPSHOTS=true bazel run %s", os.Getenv("TEST_TARGET"))
				// srcTestDirectory is the directory of the source code of the test case.
				srcTestDirectory := path.Join(buildWorkspaceDirectory, path.Dir(args.TestDataPathRelative), args.Name)
				if shouldUpdate {
					// Update stdout, stderr, exit code.
					updateExpectedConfig(t, config.Stdout, redactWorkspacePath(stdout.String(), workspaceRoot), srcTestDirectory, expectedStdoutFilename)
					updateExpectedConfig(t, config.Stderr, redactWorkspacePath(stderr.String(), workspaceRoot), srcTestDirectory, expectedStderrFilename)
					updateExpectedConfig(t, fmt.Sprintf("%d", config.ExitCode), fmt.Sprintf("%d", actualExitCode), srcTestDirectory, expectedExitCodeFilename)

					err := filepath.Walk(testdataDir, func(walkedPath string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						relativePath := strings.TrimPrefix(walkedPath, testdataDir)
						if shouldUpdate {
							if buildWorkspaceDirectory == "" {
								t.Fatalf("Tried to update snapshots but no BUILD_WORKSPACE_DIRECTORY specified.\n Try %s.", updateCommand)
							}

							if info.Name() == "BUILD.bazel" {
								destFile := strings.TrimSuffix(path.Join(buildWorkspaceDirectory, path.Dir(args.TestDataPathRelative)+relativePath), ".bazel") + args.BuildOutSuffix

								err := copyFile(walkedPath, destFile)
								if err != nil {
									t.Fatalf("Failed to copy file %v to %v. Error: %v\n", walkedPath, destFile, err)
								}
							}

						}
						t.Logf("%q exists in %v", relativePath, testdataDir)
						return nil
					})
					if err != nil {
						t.Fatalf("Failed to walk file: %v", err)
					}

				} else {
					t.Logf(`
=====================================================================================
Run %s to update BUILD.out and expected{Stdout,Stderr,ExitCode}.txt files.
=====================================================================================
`, updateCommand)
				}
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), args.Timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, args.GazelleBinaryPath, config.Args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Dir = workspaceRoot
		cmd.Env = append(os.Environ(), fmt.Sprintf("BUILD_WORKSPACE_DIRECTORY=%v", workspaceRoot))
		if err := cmd.Run(); err != nil {
			var e *exec.ExitError
			if !errors.As(err, &e) {
				t.Fatal(err)
			}
		}
		errs := make([]error, 0)
		actualExitCode = cmd.ProcessState.ExitCode()
		if config.ExitCode != actualExitCode {
			if actualExitCode == cmdTimeoutOrInterruptExitCode {
				errs = append(errs, fmt.Errorf("gazelle exceeded the timeout or was interrupted"))
			} else {

				errs = append(errs, fmt.Errorf("expected gazelle exit code: %d\ngot: %d",
					config.ExitCode, actualExitCode,
				))
			}
		}
		actualStdout := redactWorkspacePath(stdout.String(), workspaceRoot)
		if normalizeSpace(config.Stdout) != normalizeSpace(actualStdout) {
			errs = append(errs, fmt.Errorf("expected gazelle stdout: %s\ngot: %s",
				config.Stdout, actualStdout,
			))
		}
		actualStderr := redactWorkspacePath(stderr.String(), workspaceRoot)
		if normalizeSpace(config.Stderr) != normalizeSpace(actualStderr) {
			errs = append(errs, fmt.Errorf("expected gazelle stderr: %s\ngot: %s",
				config.Stderr, actualStderr,
			))
		}
		if len(errs) > 0 {
			for _, err := range errs {
				t.Log(err)
			}
			t.Fail()
		}

		CheckFiles(t, testdataDir, goldens)
	})
}

func copyFile(src string, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}
	err = destFile.Sync()
	if err != nil {
		return err
	}
	return nil
}

type testConfig struct {
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
}

// updateExpectedConfig writes to an expected stdout, stderr, or exit code file
// with the latest results of a test.
func updateExpectedConfig(t *testing.T, expected string, actual string, srcTestDirectory string, expectedFilename string) {
	if expected != actual {
		destFile := path.Join(srcTestDirectory, expectedFilename)

		err := os.WriteFile(destFile, []byte(actual), 0o644)
		if err != nil {
			t.Fatalf("Failed to write file %v. Error: %v\n", destFile, err)
		}
	}
}

// redactWorkspacePath replaces workspace path with a constant to make the test
// output reproducible.
func redactWorkspacePath(s, wsPath string) string {
	return strings.ReplaceAll(s, wsPath, "%WORKSPACEPATH%")
}

func normalizeSpace(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
