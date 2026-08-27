Unit tests for the `go_deps` module extension. Prefer adding tests here over
`//tests/bcr/go_mod` or `go_work`: faster, but less realistic (heavy mocking).
Keep new tests minimal and focused. Examples: `module.bzl` (minimal),
`bcr_go_mod.bzl` (complete; large because it was derived from
`//tests/bcr/go_mod`), `mvs.bzl` (multi-module version selection).

Each test case is a `.bzl` file that assigns a JSON string to `TEST`. See
`//internal/bzlmod:go_deps_test_case.bzl` for the schema. Use
`//tools/convert-go-deps-test-case` to convert between `.bzl` files and
scratch directories where Bazel can be run.

DO NOT WRITE BY HAND. Follow the process below and use the conversion tool.
New tests must pass `//tests/bzlmod:go_deps_test` and
`//tools/convert-go-deps-test-case:convert-go-deps-test-case_test`.

## Create a new test

1. Create a scratch directory somewhere outside of the Gazelle repo. For each
   Bazel module in the test, create a subdirectory (named after the module)
   containing a `MODULE.bazel` file:

   1. Declare the module name:

       ```
       module(name = "example_mod")
       ```

   2. Depend on this Gazelle repo:

       ```
       bazel_dep(name = "gazelle", version = "1.0.0")
       local_path_override(
           module_name = "gazelle",
           path = "/path/to/bazel-gazelle",  # your clone of this repo
       )
       ```

       When you unpack an existing test with the converter, this path is set
       automatically to the repository root.

   3. In the root module, depend on each other module with `bazel_dep` and
      `local_path_override`, using relative paths (for example,
      `path = "../other_mod"`).

   4. Load the `go_deps` module extension and declare any tags needed:

       ```
       go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
       go_deps.from_file(go_mod = "//:go.mod")
       ```

   5. In the root module, expose the config repo:

       ```
       use_repo(go_deps, "bazel_gazelle_go_repository_config")
       ```

   6. Add any other files the test needs. Include `go.mod` and/or `go.work`
      only when referenced by a `from_file` tag, along with the matching
      `.sum` file (`go.sum` or `go.work.sum`).

   7. Add an empty `BUILD` file in each module directory.

2. From the root module's subdirectory, verify that `go_deps` runs and resolves
   the same module versions as Go:

   ```
   bazel mod show_repo --all_repos
   go list -m all
   ```

   Sanity checks that the config repo was created:

   ```
   bazel query @bazel_gazelle_go_repository_config//:all
   cat $(bazel info output_base)/external/gazelle++go_deps+bazel_gazelle_go_repository_config/WORKSPACE
   ```

3. Write `comment.txt` in the scratch directory (alongside the module
   subdirectories). This becomes the docstring at the top of the `.bzl` file.

4. Write `want.json`: a JSON array of objects describing repos that `go_deps`
   should declare. Start from the `bazel mod show_repo --all_repos` output;
   keep only `go_repository` entries. Omit internal attributes and attributes
   whose values are defaults.

   If the test needs mocked `go` command output (for example when `go_deps`
   runs `go list`), create an `executions/` subdirectory. Add one file per
   command. The first line must be `# ` followed by the command string (without
   the `env -i` wrapper and with `go` instead of the GOROOT path). The
   remaining lines are the command's stdout. When unpacking an existing test,
   the converter writes these files automatically.

5. Pack the scratch directory into a `.bzl` file (the file name, without
   `.bzl`, becomes the test `name` field):

   ```
   bazel run //tools/convert-go-deps-test-case -- \
       -from=SCRATCH_DIR -to=tests/bzlmod/go_deps/NEW_TEST.bzl
   ```

6. Register the test in `tests/bzlmod/go_deps_test.bzl`: load `TEST` from the
   new file and append it to `_GO_DEPS_TEST_CASES`.

7. Run `bazel test //tests/bzlmod:go_deps_test`.

   New cases do not add separate test targets; individual cases cannot be run
   on their own. To confirm a case is exercised, temporarily put an incorrect
   value in `want` and re-run the test.

## Modify an existing test

1. Unpack the test into a scratch directory outside of the Gazelle repo. The
   output directory must be empty unless you pass `-f` to remove it first:

   ```
   bazel run //tools/convert-go-deps-test-case -- \
       -from=tests/bzlmod/go_deps/EXAMPLE.bzl -to=SCRATCH_DIR
   ```

2. Make changes, verify behavior in the scratch directory, and re-pack using
   the steps above.
