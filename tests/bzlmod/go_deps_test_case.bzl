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
  - tags_dev: optional set of go_deps tags with the same schema as tags,
    used when go_deps is loaded with 'dev_dependency = True'.
  - tags_isolate: optional set of go_deps tags with the same schema as tags,
    used when go_deps is loaded with 'isolate = True'.
- files: a dict of files. Keys are paths of the form "./module_name/file".
  Values are file contents.
- executions: optional dict mapping go_deps instance names to dicts of command
  strings to expected stdout from mocked module_ctx.execute calls. The "main"
  key is for the un-isolated go_deps instance. Additional keys have the form
  "<module_name>_isolate" for isolated instances. Command strings omit the
  leading "env -i" wrapper and use "go" instead of the GOROOT path to the go
  binary.
- want: object mapping go_deps instance names to expected output objects.
  The "main" key is for the un-isolated go_deps instance. Additional keys have
  the form "<module_name>_isolate" for isolated instances. Each value has:
  - repos: list of repos that go_deps is expected to declare. Each element is a
    dict of expected attributes. It's not an error if a declared repo or
    attributes are omitted from this list.
  - root_module_direct_deps: list of repo names passed to extension metadata as
    root_module_direct_deps.
  - root_module_direct_dev_deps: list of repo names passed to extension metadata
    as root_module_direct_dev_deps.
  - print: optional list of substrings expected to appear in messages passed to
    module_ctx.print, in order.
  - fail: optional list of substrings expected to appear in messages passed to
    module_ctx.fail, in order. If fail was called and this field is omitted,
    the test fails.

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
        executions = d.get("executions", {}),
        want = {
            key: _parse_want(value)
            for key, value in d["want"].items()
        },
    )

def _parse_want(d):
    return struct(
        repos = [_parse_want_repo(w) for w in d.get("repos", [])],
        root_module_direct_deps = d.get("root_module_direct_deps", []),
        root_module_direct_dev_deps = d.get("root_module_direct_dev_deps", []),
        print = d.get("print", []),
        fail = d.get("fail", []),
    )

def _parse_module(d):
    return struct(
        name = d["name"],
        is_root = d.get("is_root", False),
        version = d.get("version", ""),
        tags = _parse_tags(d.get("tags", {})),
        tags_dev = _parse_tags(d.get("tags_dev", {})),
        tags_isolate = _parse_tags(d.get("tags_isolate", {})),
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

def _apply_tag_defaults(d, defaults):
    """Returns a struct with all tag attributes set, like a real module extension tag."""
    attrs = {}
    for key, default in defaults.items():
        if key in d:
            attrs[key] = d[key]
        else:
            attrs[key] = default
    return struct(**attrs)

def _parse_archive_override_tag(d):
    return _apply_tag_defaults(d, {
        "path": None,
        "urls": [],
        "strip_prefix": "",
        "sha256": "",
        "patches": [],
        "patch_strip": 0,
        "patch_cmds": [],
    })

def _parse_config_tag(d):
    return _apply_tag_defaults(d, {
        "checks": "warning",
        "check_direct_dependencies": "off",
        "go_env": {},
        "go_env_inherit": [],
        "debug_mode": False,
    })

def _parse_from_file_tag(d):
    d = dict(d)
    go_mod = d.get("go_mod")
    go_work = d.get("go_work")
    if type(go_mod) == "string":
        d["go_mod"] = Label(go_mod)
    if type(go_work) == "string":
        d["go_work"] = Label(go_work)
    return _apply_tag_defaults(d, {
        "go_mod": None,
        "go_work": None,
        "fail_on_version_conflict": False,
    })

def _parse_gazelle_default_attributes_tag(d):
    return _apply_tag_defaults(d, {
        "build_file_generation": "on",
        "build_extra_args": [],
        "directives": [],
    })

def _parse_gazelle_override_tag(d):
    return _apply_tag_defaults(d, {
        "path": None,
        "build_file_generation": "on",
        "build_extra_args": [],
        "directives": [],
    })

def _parse_module_tag(d):
    return _apply_tag_defaults(d, {
        "path": None,
        "version": None,
        "sum": "",
        "indirect": False,
        "build_naming_convention": "",
        "build_file_proto_mode": "",
        "local_path": "",
    })

def _parse_module_override_tag(d):
    return _apply_tag_defaults(d, {
        "path": None,
        "patches": [],
        "patch_strip": [],
        "patch_cmds": [],
        "repo_name": "",
    })

def _parse_want_repo(d):
    return struct(**d)
