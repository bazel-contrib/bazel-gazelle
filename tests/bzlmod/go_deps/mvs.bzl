"""
Minimal version selection across Bazel modules.

The root module requires golang.org/x/sync at a low version via from_file.
A dependency Bazel module requires the same Go module at a higher version via
a module tag. go_deps should select the higher version.
"""

load(
    "//internal/bzlmod:go_deps_test_case.bzl",
    "config_tag",
    "from_file_tag",
    "module",
    "module_tag",
    "tags",
    "test_case",
)

TEST = test_case(
    name = "mvs",
    modules = [
        module(
            name = "mvs_test",
            is_root = True,
            tags = tags(
                # Silence the expected warning that the root's direct dep was
                # upgraded by MVS (v0.3.0 -> v0.11.0).
                config = [config_tag()],
                from_file = [from_file_tag(go_mod = "@@mvs_test//:go.mod")],
            ),
        ),
        module(
            name = "mvs_dep",
            version = "1.0.0",
            tags = tags(
                module = [module_tag(
                    path = "golang.org/x/sync",
                    sum = "h1:GGz8+XQP4FvTTrjZPzNKTMFtSXH80RAzG+5ghFPgK9w=",
                    version = "v0.11.0",
                )],
            ),
        ),
    ],
    files = {
        "/mvs_test/go.mod": """module example.com/mvs_test

go 1.24.12

require golang.org/x/sync v0.3.0
""",
        "/mvs_test/go.sum": """golang.org/x/sync v0.3.0 h1:ftCYgMx6zT/asHUrPw8BLLscYtGznsLAnjq5RH9P66E=
golang.org/x/sync v0.3.0/go.mod h1:FU7BRWz2tNW+3quACPkgCx/L+uEAv1htQ0V83Z9Rj+Y=
""",
    },
    want = [
        struct(
            name = "org_golang_x_sync",
            importpath = "golang.org/x/sync",
            version = "v0.11.0",
            sum = "h1:GGz8+XQP4FvTTrjZPzNKTMFtSXH80RAzG+5ghFPgK9w=",
        ),
    ],
)
