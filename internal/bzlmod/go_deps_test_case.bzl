"""
Utility functions for writing go_deps_test cases.
"""

def test_case(
        *,
        name,
        modules,
        files = {},
        want):
    """
    Constructs a test case

    Every test .bzl file should set TEST to a value returned by this function.

    Args:
        name: name of the test case, for error messages.
        modules: list of mock Bazel modules constructed with module.
            Must have at least one element. The first element must have
            `is_root = True`.
        files: dict of mock files that may be accessed with module_ctx.read,
            Keys are path strings like "/repo/pkg/file". Values are file contents.
        want: list of repos that must be declared. Each element is a struct
            containing attributes of the desired repo. The "name" attribute
            must be unique. The test allows repos and repo attributes not
            declared in this list.
    """
    return struct(
        name = name,
        modules = modules,
        files = files,
        want = want,
    )

def tags(
        *,
        archive_override = [],
        config = [],
        from_file = [],
        gazelle_override = [],
        gazelle_default_attributes = [],
        module = [],
        module_override = []):
    """
    Constructs a mock Bazel module tag set, listed in module.tags.

    Each argument is a list of tags, corresponding to tags supported by the
    go_deps module extension.
    """
    return struct(
        archive_override = archive_override,
        config = config,
        from_file = from_file,
        gazelle_override = gazelle_override,
        gazelle_default_attributes = gazelle_default_attributes,
        module = module,
        module_override = module_override,
    )

def module(
        *,
        name,
        is_root = False,
        version = "",
        tags = tags()):
    """
    Constructs a mock Bazel module, listed in module_ctx.modules

    Each argument is a field for the mock module object.
    """
    return struct(
        name = name,
        is_root = is_root,
        version = version,
        tags = tags,
    )

def archive_override_tag(
        *,
        path,
        urls = [],
        strip_prefix = "",
        sha256 = "",
        patches = [],
        patch_strip = 0,
        patch_cmds = []):
    return struct(
        path = path,
        urls = urls,
        strip_prefix = strip_prefix,
        sha256 = sha256,
        patches = patches,
        patch_strip = patch_strip,
        patch_cmds = patch_cmds,
    )

def config_tag(
        *,
        go_env = {},
        go_env_inherit = []):
    return struct(
        check_direct_dependencies = "off",
        go_env = go_env,
        go_env_inherit = go_env_inherit,
        debug_mode = False,
    )

def gazelle_default_attributes_tag(
        *,
        build_file_generation = "on",
        build_extra_args = [],
        directives = []):
    return struct(
        build_file_generation = build_file_generation,
        build_extra_args = build_extra_args,
        directives = directives,
    )

def gazelle_override_tag(
        *,
        path,
        build_file_generation = "on",
        build_extra_args = [],
        directives = []):
    return struct(
        path = path,
        build_file_generation = build_file_generation,
        build_extra_args = build_extra_args,
        directives = directives,
    )

def from_file_tag(
        *,
        go_mod = None,
        go_work = None):
    if type(go_mod) == "string":
        go_mod = Label(go_mod)
    if type(go_work) == "string":
        go_work = Label(go_work)
    return struct(
        go_mod = go_mod,
        go_work = go_work,
        fail_on_version_conflict = True,
    )

def module_tag(
        *,
        path,
        version,
        sum = "",
        indirect = False,
        build_naming_convention = "",
        build_file_proto_mode = "",
        local_path = ""):
    return struct(
        path = path,
        version = version,
        sum = sum,
        indirect = indirect,
        build_naming_convention = build_naming_convention,
        build_file_proto_mode = build_file_proto_mode,
        local_path = local_path,
    )

def module_override_tag(
        *,
        path,
        patches = [],
        patch_strip = [],
        patch_cmds = [],
        repo_name = ""):
    return struct(
        path = path,
        patches = patches,
        patch_strip = patch_strip,
        patch_cmds = patch_cmds,
        repo_name = repo_name,
    )
