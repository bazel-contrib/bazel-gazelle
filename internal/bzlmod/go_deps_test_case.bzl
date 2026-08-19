"""
Utility functions for writing go_deps_test cases.

A test case is a .bzl file in //tests/bzlmod/go_deps that assigns a
JSON string to a global variable named TEST. We use JSON because Starlark
can parse it, and we can use a separate program to convert it to/from
an actual directory where we can run build commands.

Each test case has the following fields:

- name: name of the test case, should match the file name
- modules: a list of Bazel modules (must have at least one element). The data
  here roughly matches what you'd see in module_ctx.modules.
  - name: the name of the module
  - is_root: true for the first module
  - tags: a set of go_deps tags. Each field corresponds to a tag like module
    or from_file. Each value is a list of dicts, the attributes for each tag.
- files: a dict of files. Keys are paths of the form "./module_name/file".
  Values are file contents.
- want: list of repos that go_deps is expected to declare. Each element is a
  dict of expected attributes. It's not an error if a declared repo or
  attributes are omitted from this list.

See //tests/bzlmod/go_deps:README.md for instructions on authoring tests.
"""

def parse_go_deps_test_case(s):
    """
    Parses a JSON test case spec into a test case struct.

    Args:
        s: JSON string describing the test case. Default values are applied
            to all fields after decoding.
    """
    d = json.decode(s)
    return struct(
        name = d["name"],
        modules = [_parse_module(m) for m in d["modules"]],
        files = d.get("files", {}),
        want = [_parse_want_repo(w) for w in d["want"]],
    )

def _parse_module(d):
    return struct(
        name = d["name"],
        is_root = d.get("is_root", False),
        version = d.get("version", ""),
        tags = _parse_tags(d.get("tags", {})),
    )

def _parse_tags(d):
    return struct(
        archive_override = [_parse_archive_override_tag(t) for t in d.get("archive_override", [])],
        config = [_parse_config_tag(t) for t in d.get("config", [])],
        from_file = [_parse_from_file_tag(t) for t in d.get("from_file", [])],
        gazelle_override = [_parse_gazelle_override_tag(t) for t in d.get("gazelle_override", [])],
        gazelle_default_attributes = [_parse_gazelle_default_attributes_tag(t) for t in d.get("gazelle_default_attributes", [])],
        module = [_parse_module_tag(t) for t in d.get("module", [])],
        module_override = [_parse_module_override_tag(t) for t in d.get("module_override", [])],
    )

def _parse_archive_override_tag(d):
    return struct(
        path = d["path"],
        urls = d.get("urls", []),
        strip_prefix = d.get("strip_prefix", ""),
        sha256 = d.get("sha256", ""),
        patches = d.get("patches", []),
        patch_strip = d.get("patch_strip", 0),
        patch_cmds = d.get("patch_cmds", []),
    )

def _parse_config_tag(d):
    return struct(
        check_direct_dependencies = "off",
        go_env = d.get("go_env", {}),
        go_env_inherit = d.get("go_env_inherit", []),
        debug_mode = False,
    )

def _parse_from_file_tag(d):
    go_mod = d.get("go_mod")
    go_work = d.get("go_work")
    if type(go_mod) == "string":
        go_mod = Label(go_mod)
    if type(go_work) == "string":
        go_work = Label(go_work)
    return struct(
        go_mod = go_mod,
        go_work = go_work,
        fail_on_version_conflict = True,
    )

def _parse_gazelle_default_attributes_tag(d):
    return struct(
        build_file_generation = d.get("build_file_generation", "on"),
        build_extra_args = d.get("build_extra_args", []),
        directives = d.get("directives", []),
    )

def _parse_gazelle_override_tag(d):
    return struct(
        path = d["path"],
        build_file_generation = d.get("build_file_generation", "on"),
        build_extra_args = d.get("build_extra_args", []),
        directives = d.get("directives", []),
    )

def _parse_module_tag(d):
    return struct(
        path = d["path"],
        version = d["version"],
        sum = d.get("sum", ""),
        indirect = d.get("indirect", False),
        build_naming_convention = d.get("build_naming_convention", ""),
        build_file_proto_mode = d.get("build_file_proto_mode", ""),
        local_path = d.get("local_path", ""),
    )

def _parse_module_override_tag(d):
    return struct(
        path = d["path"],
        patches = d.get("patches", []),
        patch_strip = d.get("patch_strip", []),
        patch_cmds = d.get("patch_cmds", []),
        repo_name = d.get("repo_name", ""),
    )

def _parse_want_repo(d):
    return struct(**d)
