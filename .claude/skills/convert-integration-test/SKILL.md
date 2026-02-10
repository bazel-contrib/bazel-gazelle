---
name: convert-integration-test
description: converts a test from cmd/gazelle/integration_test.go into a directory in //tests covered by gazelle_generation_test
---

Refer to the doc comment in @internal/generationtest/generationtest.bzl for the format of //tests subdirectories. A typical test case in integration_test.go creates a temporary directory with testtools.CreateFiles, calls runGazelle, then checks the output. gazelle_generation_test is designed to support cases like this, but not all cases follow this pattern.

To determine whether a test case can be converted, check that NONE of the following conditions apply:

- It relies on commands other than "fix" or "update". For example, "update-repos" and "help" tests cannot be converted.
- It lacks a WORKSPACE or MODULE.bazel file.
- The test is related to build file names (probably setting -build_file_name).
- It doesn't cleanly follow the pattern supported by gazelle_generation_test.

To convert a test case:

1. Create a //tests subdirectory named after the test case. Change the name to snake_case, drop the "Test" prefix and add a "go_" prefix.
2. Copy content of test files exactly. DO NOT MAKE CHANGES to test file content. In particular, do not change "//" to "/" in label strings.
2. Rename initial BUILD and BUILD.bazel files to BUILD.in. Do not use BUILD.bazel.in.
3. Rename expected output files from BUILD and BUILD.bazel to BUILD.out. Do not use BUILD.bazel.out.
4. Put arguments passed to runGazelle into arguments.txt.
5. Write expectedExitCode.txt if a failure is expected.
6. Write the test case's doc comment to README.md, changing the name as appropriate. Don't write a README.md if the test didn't have a doc comment.

Do not modify integration_tests.go.

Do not run the test.
