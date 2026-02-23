go_migrate_select_from_workspace_to_bzlmod tests that when a MODULE.bazel file
exists with a repo_name mapping for rules_go, Gazelle correctly migrates select
statements that reference @io_bazel_rules_go//go/platform: to use the new
repo_name (@my_rules_go//go/platform:) from the MODULE.bazel file instead of
the legacy WORKSPACE name.
