Unit tests for the `go_deps` module extension. Prefer adding tests here over `//tests/bcr/go_mod` or `go_work`: faster, but less realistic (heavy mocking). Keep new tests minimal and focused. Examples: `module.bzl` (minimal), `bcr_go_mod.bzl` (complete; large because it was derived from `//tests/bcr/go_mod`).

To create a new test:

1. Build a scratch Bazel module (`MODULE.bazel` + needed files) using `go_deps` and any required tags. Add `go.mod` / `go.work` only when referenced by a `from_file` tag; include the matching `.sum` (`go.sum` / `go.work.sum`). For multiple Bazel modules, use separate directories and `local_path_override` for non-root modules.
2. In the scratch dir, verify a package builds (`bazel build`) and that `go_deps` picked the same versions as Go (`go list -m all` vs `bazel mod show_repo --all_repos`).
3. Save `bazel mod show_repo --all_repos` output to a file (full repo description).
4. Add a `.bzl` test case in this directory from the scratch module (`module.bzl` / `bcr_go_mod.bzl` for format):
    1. Load constructors from `//internal/bzlmod:go_deps_test_case.bzl`.
    2. Declare `TEST = test_case(...)`.
    3. `name` = file name stem.
    4. `modules`: one mock module per scratch `MODULE.bazel` that uses `go_deps` (seen via `module_ctx.modules`). Translate `go_deps` tags to mocks. Labels use `@@`, e.g. `@@gazelle//:go.mod`.
    5. `files`: mock paths for `module_ctx.read`, keys like `/repo_name/path` (e.g. `/gazelle/v2/go.mod`), values = contents. Include `from_file`-referenced `go.mod` / `go.work` and their `.sum` files. Omit `MODULE.bazel` and source files.
    6. `want`: expected declared repos. Start from the `show_repo` dump; keep only `go_repository` entries; omit internal attrs and attrs at default values. Extra declared repos / unlisted attrs are ignored.
5. Register in `//tests/bzlmod:go_deps_test.bzl`: load `TEST`, append to `_GO_DEPS_TEST_CASES`.
6. Run `bazel test //tests/bzlmod:go_deps_test`. New cases do not add suite targets; individual cases cannot be run alone. To confirm a case runs, put something false in `want` and re-run.
