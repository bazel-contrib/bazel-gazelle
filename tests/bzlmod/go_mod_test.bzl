load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//internal/bzlmod:go_mod.bzl", "parse_go_mod", "parse_go_sum", "parse_go_work", "use_spec_to_label")

_GO_MOD_CONTENT = """ go 1.18

require (
  github.com/bazelbuild/buildtools v0.0.0-20220531122519-a43aed7014c8
	github.com/bazelbuild/rules_go "v0.\\n\\\\\\"33.0"
github.com/bmatcuk/doublestar/v4 v4.0.2 // indirect
	// some comment
	`golang.org/x/tools` "v0.1.11" // foobar
	github.com/go-fsnotify/fsnotify v1.5.4
)

replace github.com/go-fsnotify/fsnotify => github.com/fsnotify/fsnotify v1.4.2
replace github.com/bmatcuk/doublestar/v4 v4.0.2 => github.com/bmatcuk/doublestar/v4 v4.0.3
replace example.org/hello => ../fixtures/hello

module github.com/bazelbuild/bazel-gazelle

	exclude    (
	github.com/bazelbuild/rules_go v0.33.0
  )

  retract v1.0.0

require golang.org/x/sys v0.0.0-20220624220833-87e55d714810 // indirect

tool  golang.org/x/tools/cmd/bisect
"""

_EXPECTED_GO_MOD_PARSE_RESULT = struct(
    go = (1, 18),
    module = "github.com/bazelbuild/bazel-gazelle",
    replace_map = {
        "github.com/go-fsnotify/fsnotify": struct(local_path = None, from_version = None, to_path = "github.com/fsnotify/fsnotify", version = "1.4.2"),
        "github.com/bmatcuk/doublestar/v4": struct(local_path = None, from_version = "4.0.2", to_path = "github.com/bmatcuk/doublestar/v4", version = "4.0.3"),
        "example.org/hello": struct(local_path = "../fixtures/hello", from_version = None, to_path = "example.org/hello", version = "{"),
    },
    require = (
        struct(indirect = False, path = "github.com/bazelbuild/buildtools", version = "v0.0.0-20220531122519-a43aed7014c8"),
        struct(indirect = False, path = "github.com/bazelbuild/rules_go", version = "v0.n\\\"33.0"),
        struct(indirect = True, path = "github.com/bmatcuk/doublestar/v4", version = "v4.0.2"),
        struct(indirect = False, path = "golang.org/x/tools", version = "v0.1.11"),
        struct(indirect = False, path = "github.com/go-fsnotify/fsnotify", version = "v1.5.4"),
        struct(indirect = True, path = "golang.org/x/sys", version = "v0.0.0-20220624220833-87e55d714810"),
    ),
    tool = (
        "golang.org/x/tools/cmd/bisect",
    ),
)

def _go_mod_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(env, _EXPECTED_GO_MOD_PARSE_RESULT, parse_go_mod(_GO_MOD_CONTENT, "/go.mod"))
    return unittest.end(env)

go_mod_test = unittest.make(_go_mod_test_impl)

_GO_MOD_21_CONTENT = """go 1.21.0rc1

module example.com

toolchain go1.22.2
"""

_EXPECTED_GO_MOD_21_PARSE_RESULT = struct(
    go = (1, 21),
    module = "example.com",
    replace_map = {},
    require = (),
    tool = (),
)

def _use_spec_to_label_test_impl(ctx):
    env = unittest.begin(ctx)

    asserts.equals(env, Label("@@org_example//go_mod_one:go.mod"), use_spec_to_label("org_example", "./go_mod_one"))
    asserts.equals(env, Label("@@org_example//go_mod_one:go.mod"), use_spec_to_label("org_example", "./go_mod_one/"))
    asserts.equals(env, Label("@@org_example//bar/go_mod_one:go.mod"), use_spec_to_label("org_example", "./bar/go_mod_one"))

    return unittest.end(env)

use_spec_test = unittest.make(_use_spec_to_label_test_impl)

def _go_mod_21_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(env, _EXPECTED_GO_MOD_21_PARSE_RESULT, parse_go_mod(_GO_MOD_21_CONTENT, "/go.mod"))
    return unittest.end(env)

go_mod_21_test = unittest.make(_go_mod_21_test_impl)

_GO_MOD_GODEBUG_CONTENT = """go 1.24.6

module example.com

godebug (
    randseednop=0
    rsa1024min=0
)
"""

_EXPECTED_GO_MOD_GODEBUG_PARSE_RESULT = struct(
    go = (1, 24),
    module = "example.com",
    replace_map = {},
    require = (),
    tool = (),
)

def _go_mod_godebug_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(env, _EXPECTED_GO_MOD_GODEBUG_PARSE_RESULT, parse_go_mod(_GO_MOD_GODEBUG_CONTENT, "/go.mod"))
    return unittest.end(env)

go_mod_godebug_test = unittest.make(_go_mod_godebug_test_impl)

_GO_SUM_CONTENT = """cloud.google.com/go v0.26.0/go.mod h1:aQUYkXzVsufM+DwF1aE+0xfcU+56JwCaLick0ClmMTw=
github.com/BurntSushi/toml v0.3.1/go.mod h1:xHWCNGjB5oqiDr8zfno3MHue2Ht5sIBksp03qcyfWMU=
""" + """<<<<<<<
github.com/bazelbuild/buildtools v0.0.0-20220531122519-a43aed7014c8 h1:fmdo+fvvWlhldUcqkhAMpKndSxMN3vH5l7yow5cEaiQ=
github.com/bazelbuild/buildtools v0.0.0-20220531122519-a43aed7014c8/go.mod h1:689QdV3hBP7Vo9dJMmzhoYIyo/9iMhEmHkJcnaPRCbo=
""" + """=======
github.com/bazelbuild/buildtools2 v0.0.0-20220531122519-a43aed7014c8 h1:fmdo+fvvWlhldUcqkhAMpKndSxMN3vH5l7yow5cEaiQ=
github.com/bazelbuild/buildtools2 v0.0.0-20220531122519-a43aed7014c8/go.mod h1:689QdV3hBP7Vo9dJMmzhoYIyo/9iMhEmHkJcnaPRCbo=
""" + """>>>>>>>
github.com/bazelbuild/rules_go v0.33.0 h1:WW9CHmFxbE+Lm4qiLOFAPogmiAUzZtvQsWxUcm4wwaU=
github.com/bazelbuild/rules_go v0.33.0/go.mod h1:MC23Dc/wkXEyk3Wpq6lCqz0ZAYOZDw2DR5y3N1q2i7M=
"""

_EXPECTED_GO_SUM_PARSE_RESULT = {
    ("github.com/bazelbuild/buildtools", "0.0.0-20220531122519-a43aed7014c8"): "h1:fmdo+fvvWlhldUcqkhAMpKndSxMN3vH5l7yow5cEaiQ=",
    ("github.com/bazelbuild/buildtools2", "0.0.0-20220531122519-a43aed7014c8"): "h1:fmdo+fvvWlhldUcqkhAMpKndSxMN3vH5l7yow5cEaiQ=",
    ("github.com/bazelbuild/rules_go", "0.33.0"): "h1:WW9CHmFxbE+Lm4qiLOFAPogmiAUzZtvQsWxUcm4wwaU=",
}

def _go_sum_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(env, _EXPECTED_GO_SUM_PARSE_RESULT, parse_go_sum(_GO_SUM_CONTENT))
    return unittest.end(env)

go_sum_test = unittest.make(_go_sum_test_impl)

_GO_WORK_CONTENT = """go 1.18
use ./go_mod_one
use (
    ./foo/go_mod_two
    ./bar/baz/go_mod_three
)

replace github.com/go-fsnotify/fsnotify => github.com/fsnotify/fsnotify v1.4.2
replace github.com/bmatcuk/doublestar/v4 v4.0.2 => github.com/bmatcuk/doublestar/v4 v4.0.3
replace example.org/hello => ../fixtures/hello
"""

_EXPECTED_GO_WORK_PARSE_RESULT = struct(
    go = (1, 18),
    from_file_tags = [
        struct(_is_dev_dependency = False, go_mod = Label("//go_mod_one:go.mod")),
        struct(_is_dev_dependency = False, go_mod = Label("//foo/go_mod_two:go.mod")),
        struct(_is_dev_dependency = False, go_mod = Label("//bar/baz/go_mod_three:go.mod")),
    ],
    module_tags = [
        struct(indirect = False, _parent_label = Label("//:go.work"), local_path = None, path = "github.com/fsnotify/fsnotify", version = "1.4.2"),
        struct(indirect = False, _parent_label = Label("//:go.work"), local_path = None, path = "github.com/bmatcuk/doublestar/v4", version = "4.0.3"),
        struct(indirect = False, _parent_label = Label("//:go.work"), local_path = "../fixtures/hello", path = "example.org/hello", version = "{"),
    ],
    replace_map = {
        "github.com/go-fsnotify/fsnotify": struct(local_path = None, from_version = None, to_path = "github.com/fsnotify/fsnotify", version = "1.4.2"),
        "github.com/bmatcuk/doublestar/v4": struct(local_path = None, from_version = "4.0.2", to_path = "github.com/bmatcuk/doublestar/v4", version = "4.0.3"),
        "example.org/hello": struct(local_path = "../fixtures/hello", from_version = None, to_path = "example.org/hello", version = "{"),
    },
    use = [
        "./go_mod_one",
        "./foo/go_mod_two",
        "./bar/baz/go_mod_three",
    ],
)

def _go_work_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(env, _EXPECTED_GO_WORK_PARSE_RESULT, parse_go_work(_GO_WORK_CONTENT, Label("@@//:go.work")))
    return unittest.end(env)

go_work_test = unittest.make(_go_work_test_impl)

_GO_WORK_GODEBUG_CONTENT = """go 1.24
use (
    ./foo/go_mod_one
    ./bar/baz/go_mod_two
)

godebug (
    randseednop=0
    rsa1024min=0
)
"""

_EXPECTED_GO_WORK_GODEBUG_PARSE_RESULT = struct(
    go = (1, 24),
    from_file_tags = [
        struct(_is_dev_dependency = False, go_mod = Label("//foo/go_mod_one:go.mod")),
        struct(_is_dev_dependency = False, go_mod = Label("//bar/baz/go_mod_two:go.mod")),
    ],
    module_tags = [],
    replace_map = {},
    use = [
        "./foo/go_mod_one",
        "./bar/baz/go_mod_two",
    ],
)

def _go_work_godebug_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(env, _EXPECTED_GO_WORK_GODEBUG_PARSE_RESULT, parse_go_work(_GO_WORK_GODEBUG_CONTENT, Label("@@//:go.work")))
    return unittest.end(env)

go_work_godebug_test = unittest.make(_go_work_godebug_test_impl)

def go_mod_test_suite(name):
    unittest.suite(
        name,
        go_mod_test,
        go_mod_21_test,
        go_mod_godebug_test,
        go_sum_test,
        go_work_test,
        go_work_godebug_test,
        use_spec_test,
    )
