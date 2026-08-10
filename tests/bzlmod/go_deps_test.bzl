load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("@rules_testing//lib:truth.bzl", "subjects", "truth")
load("//internal/bzlmod:go_deps.bzl", "get_repo_name", "go_deps_impl")
load("//tests/bzlmod/go_deps:bcr_go_mod.bzl", BCR_GO_MOD_TEST = "TEST")
load("//tests/bzlmod/go_deps:empty.bzl", EMPTY_TEST = "TEST")
load("//tests/bzlmod/go_deps:module.bzl", MODULE_TEST = "TEST")
load("//tests/bzlmod/go_deps:mvs.bzl", MVS_TEST = "TEST")

_GO_DEPS_TEST_CASES = [
    BCR_GO_MOD_TEST,
    EMPTY_TEST,
    MODULE_TEST,
    MVS_TEST,
]

def _struct_to_dict(s):
    return {key: getattr(s, key) for key in dir(s)}

def _go_deps_test_impl(ctx):
    env = unittest.begin(ctx)
    expect = truth.expect(env)

    for case in _GO_DEPS_TEST_CASES:
        module_ctx = _mock_module_ctx(case)
        go_deps_impl(module_ctx)

        case_expect = expect.where(test_case = case.name)
        case_expect.that_collection(
            module_ctx._repos.keys(),
            expr = "declared repos",
        ).contains_at_least([repo.name for repo in case.want])

        for want_repo in case.want:
            if want_repo.name not in module_ctx._repos:
                continue
            case_expect.that_value(
                _struct_to_dict(module_ctx._repos[want_repo.name]),
                factory = subjects.dict,
                expr = "repo({})".format(want_repo.name),
            ).contains_at_least(_struct_to_dict(want_repo))

    return unittest.end(env)

go_deps_test = unittest.make(_go_deps_test_impl)

def _mock_module_ctx(case):
    repos = {}
    return struct(
        modules = case.modules,
        os = struct(
            arch = "arm64",
            environ = {},
            name = "linux",
        ),
        path = lambda v: _mock_module_ctx_path(case, v),
        read = lambda filename: _mock_module_ctx_read(case, filename),
        is_dev_dependency = lambda tag: False,
        declare_repo = lambda rule, *, name, **kwargs: _mock_module_ctx_declare_repo(repos, rule, name = name, **kwargs),
        _repos = repos,
    )

def _mock_module_ctx_path(case, v):
    repo_name = case.modules[0].name
    if type(v) == "struct" and hasattr(v, "_name"):
        # mock path, created with _mock_path. Return as-is.
        return v
    elif type(v) == "Label":
        return _mock_path(case, _label_to_path(repo_name, v))
    elif type(v) == "string":
        path_name = "/{}/{}".format(repo_name, v)
        return _mock_path(case, path_name)
    else:
        fail("can't call module_ctx.path on value {} of unknown type {}".format(v, type(v)))

def _label_to_path(default_repo_name, label):
    repo_name = label.repo_name if label.repo_name else default_repo_name
    if label.package:
        return "/{}/{}/{}".format(repo_name, label.package, label.name)
    else:
        return "/{}/{}".format(repo_name, label.name)

def _mock_module_ctx_read(case, path):
    if hasattr(path, "_name"):
        # _mock_path
        filename = path._name
    elif type(path) == "Label":
        filename = _label_to_path(case.modules[0].name, path)
    else:
        fail("can't read from file with value {} of unknown type {}".format(path, type(path)))
    if filename not in case.files:
        fail("file '{}' not included in test case".format(filename))
    return case.files[filename]

def _mock_path(case, name):
    # When we construct a path, we also need to construct every prefix that
    # could be accessed through dirname, which is syntactically accessed as
    # an attribute, not a method call. In Starlark, we don't have a way to
    # define a "getter" for dirname, so we just need to compute its value
    # ahead of time. No recursion, so we need to do this with a loop.
    if not name.startswith("/"):
        fail("name '{}' does not start with '/'".format(name))
    parts = name[1:].split("/")
    path = _make_mock_path(case, "/", "", None, True, True)
    if len(parts) > 1:
        for part in parts[:-1]:
            path = _make_mock_path(
                case,
                name = path._name + "/" + part,
                basename = part,
                dirname = path,
                exists = True,
                is_dir = True,
            )
    if len(parts) > 0:
        path = _make_mock_path(
            case,
            name = name,
            basename = parts[-1],
            dirname = path,
            exists = name in case.files or name + "/" in case.files,
            is_dir = name + "/" in case.files,
        )
    return path

def _make_mock_path(case, name, basename, dirname, exists, is_dir):
    return struct(
        _name = name,
        basename = basename,
        dirname = dirname,
        exists = exists,
        is_dir = is_dir,
        get_child = lambda *relative_paths: _mock_path(case, "/".join([name] + list(relative_paths))),
        readdir = lambda: fail("unimplemented"),
        realpath = None,  # unimplemented
    )

def _mock_module_ctx_declare_repo(repos, rule, *, name, **kwargs):
    if name in repos:
        fail("repo '{}' declared multiple times".format(name))
    repos[name] = struct(name = name, **kwargs)

def _get_repo_name_default_test_impl(ctx):
    env = unittest.begin(ctx)

    # Without any override, the repo name is derived from the import path.
    asserts.equals(
        env,
        "com_example_foo_bar_baz",
        get_repo_name("example.com/foo/bar/baz", {}),
    )
    asserts.equals(
        env,
        "com_example_foo_bar_baz",
        get_repo_name("example.com/foo_bar_baz", {}),
    )

    # An override without a repo_name falls back to the derived name.
    asserts.equals(
        env,
        "com_example_foo_bar_baz",
        get_repo_name("example.com/foo/bar/baz", {
            "example.com/foo/bar/baz": struct(repo_name = ""),
        }),
    )

    # An override for a different path does not affect this one.
    asserts.equals(
        env,
        "com_example_foo_bar_baz",
        get_repo_name("example.com/foo/bar/baz", {
            "example.com/other": struct(repo_name = "custom_name"),
        }),
    )

    return unittest.end(env)

get_repo_name_default_test = unittest.make(_get_repo_name_default_test_impl)

def _get_repo_name_override_test_impl(ctx):
    env = unittest.begin(ctx)

    # A non-empty repo_name override is used verbatim, breaking the collision
    # with another module that mangles to the same default name.
    asserts.equals(
        env,
        "com_example_foo_bar_baz_alt",
        get_repo_name("example.com/foo_bar_baz", {
            "example.com/foo_bar_baz": struct(repo_name = "com_example_foo_bar_baz_alt"),
        }),
    )

    return unittest.end(env)

get_repo_name_override_test = unittest.make(_get_repo_name_override_test_impl)

def go_deps_test_suite(name):
    unittest.suite(
        name,
        go_deps_test,
        get_repo_name_default_test,
        get_repo_name_override_test,
    )
