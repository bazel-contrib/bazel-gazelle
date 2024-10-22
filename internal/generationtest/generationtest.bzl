# Copyright 2023 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
Test for generating rules from gazelle.
"""

load("@io_bazel_rules_go//go:def.bzl", "go_test")

def gazelle_generation_test(name, gazelle_binary, test_data, build_in_suffix = ".in", build_out_suffix = ".out", gazelle_timeout_seconds = 2, size = None, **kwargs):
    """
    gazelle_generation_test is a macro for testing gazelle against workspaces.

    The generation test expects a file structure like the following:

    ```
    |-- <testDataPath>
        |-- some_test
            |-- WORKSPACE and/or MODULE.bazel -> Indicates the directory is a test case.
            |-- README.md --> README describing what the test does.
            |-- arguments.txt --> newline delimited list of arguments to pass in (ignored if empty).
            |-- expectedStdout.txt --> Expected stdout for this test.
            |-- expectedStderr.txt --> Expected stderr for this test.
            |-- expectedExitCode.txt --> Expected exit code for this test.
            |-- app
                |-- sourceFile.foo
                |-- BUILD.in --> BUILD file prior to running gazelle.
                |-- BUILD.out --> BUILD file expected after running gazelle.
    ```

    To update the expected files, run `UPDATE_SNAPSHOTS=true bazel run //path/to:the_test_target`.

    Args:
        name: The name of the test.
        gazelle_binary: The name of the gazelle binary target. For example, //path/to:my_gazelle.
        test_data: A list of target of the test data files you will pass to the test.
            This can be a https://bazel.build/reference/be/general#filegroup.
        build_in_suffix: The suffix for the input BUILD.bazel files. Defaults to .in.
            By default, will use files named BUILD.in as the BUILD files before running gazelle.
        build_out_suffix: The suffix for the expected BUILD.bazel files after running gazelle. Defaults to .out.
            By default, will use files named check the results of the gazelle run against files named BUILD.out.
        gazelle_timeout_seconds: Number of seconds to allow the gazelle process to run before killing.
        size: Specifies a test target's "heaviness": how much time/resources it needs to run.
        **kwargs: Attributes that are passed directly to the test declaration.
    """
    go_test(
        name = name,
        embed = [Label(":generationtest_test")],
        args = [
            "-gazelle_binary_path=$(rootpath %s)" % gazelle_binary,
            "-build_in_suffix=%s" % build_in_suffix,
            "-build_out_suffix=%s" % build_out_suffix,
            "-timeout=%ds" % gazelle_timeout_seconds,
        ],
        size = size,
        data = test_data + [
            gazelle_binary,
        ],
        **kwargs
    )
