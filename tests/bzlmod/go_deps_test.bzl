load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("@rules_testing//lib:truth.bzl", "matching", "subjects", "truth")
load("//internal/bzlmod:go_deps.bzl", "go_deps_impl")
load("//tests/bzlmod:go_deps_test_case.bzl", "parse_go_deps_test_case")
load("//tests/bzlmod/go_deps:archive_override.bzl", ARCHIVE_OVERRIDE_TEST = "TEST")
load("//tests/bzlmod/go_deps:archive_override_unmatched.bzl", ARCHIVE_OVERRIDE_UNMATCHED_TEST = "TEST")
load("//tests/bzlmod/go_deps:bcr_go_mod.bzl", BCR_GO_MOD_TEST = "TEST")
load("//tests/bzlmod/go_deps:build_naming_conventions.bzl", BUILD_NAMING_CONVENTIONS_TEST = "TEST")
load("//tests/bzlmod/go_deps:debug_mode.bzl", DEBUG_MODE_TEST = "TEST")
load("//tests/bzlmod/go_deps:default_gazelle_overrides.bzl", DEFAULT_GAZELLE_OVERRIDES_TEST = "TEST")
load("//tests/bzlmod/go_deps:dep_files.bzl", DEP_FILES_TEST = "TEST")
load("//tests/bzlmod/go_deps:duplicate_module_tag.bzl", DUPLICATE_MODULE_TAG_TEST = "TEST")
load("//tests/bzlmod/go_deps:empty.bzl", EMPTY_TEST = "TEST")
load("//tests/bzlmod/go_deps:gazelle_default_attributes.bzl", GAZELLE_DEFAULT_ATTRIBUTES_TEST = "TEST")
load("//tests/bzlmod/go_deps:gazelle_override.bzl", GAZELLE_OVERRIDE_TEST = "TEST")
load("//tests/bzlmod/go_deps:go_version_low.bzl", GO_VERSION_LOW_TEST = "TEST")
load("//tests/bzlmod/go_deps:isolate.bzl", ISOLATE_TEST = "TEST")
load("//tests/bzlmod/go_deps:missing_sum.bzl", MISSING_SUM_TEST = "TEST")
load("//tests/bzlmod/go_deps:module.bzl", MODULE_TEST = "TEST")
load("//tests/bzlmod/go_deps:module_dev_deps.bzl", MODULE_DEV_DEPS_TEST = "TEST")
load("//tests/bzlmod/go_deps:module_local_path.bzl", MODULE_LOCAL_PATH_TEST = "TEST")
load("//tests/bzlmod/go_deps:module_override.bzl", MODULE_OVERRIDE_TEST = "TEST")
load("//tests/bzlmod/go_deps:module_tag_version_normalize.bzl", MODULE_TAG_VERSION_NORMALIZE_TEST = "TEST")
load("//tests/bzlmod/go_deps:mvs.bzl", MVS_TEST = "TEST")
load("//tests/bzlmod/go_deps:replace_dir_mod.bzl", REPLACE_DIR_MOD_TEST = "TEST")
load("//tests/bzlmod/go_deps:replace_dir_work.bzl", REPLACE_DIR_WORK_TEST = "TEST")
load("//tests/bzlmod/go_deps:replace_ignore_not_root.bzl", REPLACE_IGNORE_NOT_ROOT_TEST = "TEST")
load("//tests/bzlmod/go_deps:replace_version_mod.bzl", REPLACE_VERSION_MOD_TEST = "TEST")
load("//tests/bzlmod/go_deps:replace_version_work.bzl", REPLACE_VERSION_WORK_TEST = "TEST")
load("//tests/bzlmod/go_deps:repo_name_collision.bzl", REPO_NAME_COLLISION_TEST = "TEST")
load("//tests/bzlmod/go_deps:rules_proto_compat.bzl", RULES_PROTO_COMPAT_TEST = "TEST")
load("//tests/bzlmod/go_deps:tool.bzl", TOOL_TEST = "TEST")
load("//tests/bzlmod/go_deps:version_conflict_checks.bzl", VERSION_CONFLICT_CHECKS_TEST = "TEST")
load("//tests/bzlmod/go_deps:workspace_mvs_pruning.bzl", WORKSPACE_MVS_PRUNING_TEST = "TEST")

# TODO(#2344): enable skipped tests for new go_deps implementation
#
# Blockers with the old implementation
# - go_deps_impl calls fail directly, so tests can't check failures
# - gazelle_override overrides settings on attributes not explicitly set
# - dep_files not set correctly
# - root_module_direct_deps not set in isolated go_deps
# - root_module_direct_deps includes indirect requirements
# - go_deps may select higher versions than before (bug fix)

# Keep sorted
_GO_DEPS_TEST_CASES = [
    ARCHIVE_OVERRIDE_TEST,
    # ARCHIVE_OVERRIDE_UNMATCHED_TEST,
    BUILD_NAMING_CONVENTIONS_TEST,
    BCR_GO_MOD_TEST,
    DEBUG_MODE_TEST,
    DEFAULT_GAZELLE_OVERRIDES_TEST,
    # DEP_FILES_TEST,
    # DUPLICATE_MODULE_TAG_TEST,
    EMPTY_TEST,
    # GAZELLE_DEFAULT_ATTRIBUTES_TEST,
    GAZELLE_OVERRIDE_TEST,
    # GO_VERSION_LOW_TEST,
    # ISOLATE_TEST,
    MODULE_DEV_DEPS_TEST,
    MODULE_LOCAL_PATH_TEST,
    MODULE_OVERRIDE_TEST,
    MODULE_TAG_VERSION_NORMALIZE_TEST,
    MODULE_TEST,
    MISSING_SUM_TEST,
    MVS_TEST,
    REPLACE_DIR_MOD_TEST,
    # REPLACE_DIR_WORK_TEST,
    REPLACE_IGNORE_NOT_ROOT_TEST,
    REPLACE_VERSION_MOD_TEST,
    # REPLACE_VERSION_WORK_TEST,
    # REPO_NAME_COLLISION_TEST,
    RULES_PROTO_COMPAT_TEST,
    TOOL_TEST,
    # VERSION_CONFLICT_CHECKS_TEST,
    # WORKSPACE_MVS_PRUNING_TEST,
]

def _go_deps_test_impl(ctx):
    env = unittest.begin(ctx)
    expect = truth.expect(env)

    for case_json in _GO_DEPS_TEST_CASES:
        case = parse_go_deps_test_case(case_json)
        _run_go_deps_instance(env, expect, case, "main", False)
        for module in case.modules:
            if _tags_empty(module.tags_isolate):
                continue
            instance_name = module.name + "_isolate"
            _run_go_deps_instance(env, expect, case, instance_name, True, module)

    return unittest.end(env)

def _run_go_deps_instance(env, expect, case, instance_name, isolated, isolate_module = None):
    if instance_name not in case.want:
        if isolated:
            fail("test case {}: missing want for isolated instance {}".format(case.name, instance_name))

    want = case.want[instance_name]
    executions = case.executions.get(instance_name, {})
    module_ctx = _mock_module_ctx(case, executions, isolated, isolate_module)
    metadata = go_deps_impl(module_ctx)
    case_expect = expect.where(test_case = case.name, instance = instance_name)

    if want.fail:
        asserts.equals(env, None, metadata)
        _assert_messages_contain_substrings(
            case_expect,
            module_ctx._state.failed_messages,
            want.fail,
            "failed_messages",
        )
    else:
        if module_ctx._state.failed_messages:
            fail("test case {} ({}): unexpected fail messages: {}".format(
                case.name,
                instance_name,
                module_ctx._state.failed_messages,
            ))
        if metadata == None:
            fail("test case {} ({}): go_deps_impl did not return extension metadata".format(case.name, instance_name))

    if want.print:
        _assert_messages_contain_substrings(
            case_expect,
            module_ctx._state.printed_messages,
            want.print,
            "printed_messages",
        )

    if metadata == None:
        return

    case_expect.that_value(
        metadata.reproducible,
        factory = subjects.bool,
        expr = "reproducible",
    ).equals(True)
    case_expect.that_collection(
        metadata.root_module_direct_deps,
        expr = "root_module_direct_deps",
    ).contains_exactly(want.root_module_direct_deps)
    case_expect.that_collection(
        metadata.root_module_direct_dev_deps,
        expr = "root_module_direct_dev_deps",
    ).contains_exactly(want.root_module_direct_dev_deps)
    case_expect.that_collection(
        module_ctx._state.repos.keys(),
        expr = "declared repos",
    ).contains_at_least([repo.name for repo in want.repos])

    for want_repo in want.repos:
        if want_repo.name not in module_ctx._state.repos:
            continue
        case_expect.that_value(
            _struct_to_dict(module_ctx._state.repos[want_repo.name]),
            factory = subjects.dict,
            expr = "repo({})".format(want_repo.name),
        ).contains_at_least(_struct_to_dict(want_repo))

go_deps_test = unittest.make(_go_deps_test_impl)

def go_deps_test_suite(name):
    unittest.suite(
        name,
        go_deps_test,
    )

def _mock_module_ctx(case, executions, isolated, isolate_module = None):
    state = struct(
        case = case,
        repos = {},
        environ = {},
        files = {},
        executions = executions,
        printed_messages = [],
        failed_messages = [],
    )
    environ = {}
    if isolated:
        modules = [_mock_isolated_module(isolate_module)]
    else:
        modules = [_mock_module(m) for m in case.modules]
    return struct(
        modules = modules,
        is_isolated = isolated,
        os = struct(
            arch = "arm64",
            environ = environ,
            name = "linux",
        ),
        execute = lambda arguments, environment = {}: _mock_module_ctx_execute(state, arguments, environment),
        declare_repo = lambda rule, *, name, **kwargs: _mock_module_ctx_declare_repo(state, rule, name = name, **kwargs),
        file = lambda path, content = "", executable = True: _mock_module_ctx_file(state, path, content),
        extension_metadata = lambda **kwargs: _mock_module_ctx_extension_metadata(**kwargs),
        getenv = state.environ.get,
        is_dev_dependency = lambda tag: getattr(tag, "_is_dev_dependency", False),
        path = lambda v: _mock_module_ctx_path(case, v),
        print = lambda msg: _mock_module_ctx_print(state, msg),
        fail = lambda msg: _mock_module_ctx_fail(state, msg),
        failed = lambda: len(state.failed_messages) > 0,
        read = lambda filename: _mock_module_ctx_read(state, filename),
        report_progress = lambda status: None,
        _state = state,
    )

def _mock_module_ctx_print(state, msg):
    state.printed_messages.append(msg)

def _mock_module_ctx_fail(state, msg):
    state.failed_messages.append(msg)

def _mock_isolated_module(module):
    return struct(
        name = module.name,
        is_root = False,
        version = module.version,
        tags = module.tags_isolate,
    )

def _mock_module(m):
    return struct(
        name = m.name,
        is_root = m.is_root,
        version = m.version,
        tags = _combine_tags(m.tags, m.tags_dev),
    )

def _combine_tags(tags, tags_dev):
    return struct(
        archive_override = tags.archive_override + _mark_dev_tags(tags_dev.archive_override),
        config = tags.config + _mark_dev_tags(tags_dev.config),
        from_file = tags.from_file + _mark_dev_tags(tags_dev.from_file),
        gazelle_override = tags.gazelle_override + _mark_dev_tags(tags_dev.gazelle_override),
        gazelle_default_attributes = tags.gazelle_default_attributes + _mark_dev_tags(tags_dev.gazelle_default_attributes),
        module = tags.module + _mark_dev_tags(tags_dev.module),
        module_override = tags.module_override + _mark_dev_tags(tags_dev.module_override),
    )

def _mark_dev_tags(tags):
    marked = []
    for tag in tags:
        fields = {key: getattr(tag, key) for key in dir(tag) if not key.startswith("_")}
        fields["_is_dev_dependency"] = True
        marked.append(struct(**fields))
    return marked

def _mock_module_ctx_execute(state, arguments, environment):
    if len(environment) > 0:
        fail("test case {}: environment was not empty; expected all environment variables to be passed through 'env -i'".format(state.case.name))
    if len(arguments) < 3 or arguments[0] != "env" or arguments[1] != "-i":
        fail("test case {}: executions expected to start with 'env -i'".format(state.case.name))
    arguments = arguments[2:]
    env_arguments = []
    goroot = ""
    for i, argument in enumerate(arguments):
        if "=" not in argument:
            arguments = arguments[i:]
            break
        if argument.startswith("GOROOT="):
            goroot = argument[len("GOROOT="):]
        if not (argument.startswith("GOROOT=") or
                argument.startswith("GOPATH=") or
                argument.startswith("GOCACHE=")):
            env_arguments.append(argument)
    if arguments[0] == goroot + "/bin/go":
        arguments[0] = "go"

    cmd_without_env = " ".join(arguments)
    cmd = " ".join(env_arguments + arguments)
    if cmd_without_env == "go version":
        # Test cases don't need to include this command, since it would be
        # the same for every one.
        return struct(
            return_code = 0,
            stdout = "go version go1.27rc3 darwin/arm64",
            stderr = "",
        )
    if cmd in state.executions:
        stdout = state.executions[cmd]
    elif cmd_without_env in state.executions:
        stdout = state.executions[cmd_without_env]
    else:
        fail("test case {}: command '{}' not included in test case".format(state.case.name, cmd))
    return struct(
        return_code = 0,
        stdout = stdout,
        stderr = "",
    )

def _mock_module_ctx_file(state, path, content):
    if hasattr(path, "_name"):
        # _mock_path
        filename = path._name
    elif type(path) == "Label":
        filename = _label_to_path(state.case.modules[0].name, path)
    elif type(path) == "string":
        filename = path
    else:
        fail("test case {}: can't read from file with value {} of unknown type {}".format(state.case.name, path, type(path)))
    state.files[filename] = content

def _mock_module_ctx_path(case, v):
    if type(v) == "struct" and hasattr(v, "_name"):
        # mock path, created with _mock_path. Return as-is.
        return v
    elif type(v) == "Label":
        default_repo_name = case.modules[0].name
        return _mock_path(case, _label_to_path(default_repo_name, v))
    elif type(v) == "string":
        return _mock_path(case, "./" + v)
    else:
        fail("test case {}: can't call module_ctx.path on value {} of unknown type {}".format(case.name, v, type(v)))

def _label_to_path(default_repo_name, label):
    repo_name = label.repo_name if label.repo_name else default_repo_name
    if label.package:
        return "./{}/{}/{}".format(repo_name, label.package, label.name)
    else:
        return "./{}/{}".format(repo_name, label.name)

def _mock_module_ctx_read(state, path):
    if hasattr(path, "_name"):
        # _mock_path
        filename = path._name
    elif type(path) == "Label":
        filename = _label_to_path(state.case.modules[0].name, path)
    else:
        fail("test case {}: can't read from file with value {} of unknown type {}".format(state.case.name, path, type(path)))
    if filename.endswith("/go.env"):
        # special case: mock @bazel_gazelle_go_repository_cache//:go.env
        # We'll get a label with mangled repo name, but we don't want to simulate
        # the mangling, so only match go.env here.
        return "GOROOT=@go_sdk//:ROOT"
    if filename in state.files:
        # file written with module_ctx.file
        return state.files[filename]
    if filename in state.case.files:
        # file from test case
        return state.case.files[filename]
    fail("test case {}: file '{}' not included in test case".format(state.case.name, filename))

def _mock_path(case, name):
    # When we construct a path, we also need to construct every prefix that
    # could be accessed through dirname, which is syntactically accessed as
    # an attribute, not a method call. In Starlark, we don't have a way to
    # define a "getter" for dirname, so we just need to compute its value
    # ahead of time. No recursion, so we need to do this with a loop.
    if not name.startswith("./"):
        fail("name '{}' does not start with './'".format(name))
    parts = name[2:].split("/")
    path = _make_mock_path(case, ".", "", None, True, True)
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

def _mock_module_ctx_declare_repo(state, rule, *, name, **kwargs):
    if name in state.repos:
        fail("test case {}: repo '{}' declared multiple times".format(state.case.name, name))
    state.repos[name] = struct(name = name, **kwargs)

def _mock_module_ctx_extension_metadata(*, root_module_direct_deps = None, root_module_direct_dev_deps = None, **kwargs):
    return struct(
        root_module_direct_deps = root_module_direct_deps if root_module_direct_deps != None else [],
        root_module_direct_dev_deps = root_module_direct_dev_deps if root_module_direct_dev_deps != None else [],
        reproducible = kwargs.get("reproducible", False),
    )

def _tags_empty(tags):
    return (
        len(tags.archive_override) == 0 and
        len(tags.config) == 0 and
        len(tags.from_file) == 0 and
        len(tags.gazelle_override) == 0 and
        len(tags.gazelle_default_attributes) == 0 and
        len(tags.module) == 0 and
        len(tags.module_override) == 0
    )

def _struct_to_dict(s):
    return {key: getattr(s, key) for key in dir(s)}

def _assert_messages_contain_substrings(case_expect, actual_messages, expected_substrings, expr):
    case_expect.that_collection(
        actual_messages,
        expr = expr,
    ).contains_exactly_predicates([
        _message_contains_substring(substring)
        for substring in expected_substrings
    ]).in_order()

def _message_contains_substring(substring):
    return matching.custom(
        'contains "{}"'.format(substring),
        lambda message: substring in message,
    )
