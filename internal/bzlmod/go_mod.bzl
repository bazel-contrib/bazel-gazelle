# Copyright 2023 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load(":semver.bzl", "COMPARES_HIGHEST_SENTINEL")

visibility([
    "//tests/bzlmod/...",
])

def _validate_go_version(path, state, tokens, line_no):
    if len(tokens) == 1:
        fail("{}:{}: expected another token after 'go'".format(path, line_no))
    if state["go"] != None:
        fail("{}:{}: unexpected second 'go' directive".format(path, line_no))
    if len(tokens) > 2:
        fail("{}:{}: unexpected token '{}' after '{}'".format(path, line_no, tokens[2], tokens[1]))

def use_spec_to_label(repo_name, use_directive):
    if use_directive.startswith("../") or "/../" in use_directive or use_directive.endswith("/.."):
        fail("go.work use directive: '{}' contains '..' which is not currently supported.".format(use_directive))

    if use_directive.startswith("/"):
        fail("go.work use directive: '{}' is an absolute path, which is not currently supported.".format(use_directive))

    if use_directive.startswith("./"):
        use_directive = use_directive[2:]

    if use_directive.endswith("/"):
        use_directive = use_directive[:-1]

    if use_directive == ".":
        use_directive = ""

    return Label("@@{}//{}:go.mod".format(repo_name, use_directive))

def go_work_from_label(module_ctx, go_work_label):
    """Loads deps from a go.work file"""
    go_work_path = module_ctx.path(go_work_label)
    go_work_content = module_ctx.read(go_work_path)
    go_work = parse_go_work(go_work_content, go_work_label)

    return _relativize_replace_paths(go_work, go_work_path)

def parse_go_work(content, go_work_label):
    # see: https://go.dev/ref/mod#go-work-file

    # Valid directive values understood by this parser never contain tabs or
    # carriage returns, so we can simplify the parsing below by canonicalizing
    # whitespace upfront.
    content = content.replace("\t", " ").replace("\r", " ")

    state = {
        "go": None,
        "use": [],
        "replace": {},
    }

    current_directive = None
    for line_no, line in enumerate(content.splitlines(), 1):
        tokens, _ = _tokenize_line(line, go_work_label.name, line_no)

        if not tokens:
            continue

        if current_directive:
            if tokens[0] == ")":
                current_directive = None
            elif current_directive == "use":
                state["use"].append(tokens[0])
            elif current_directive == "replace":
                _parse_replace_directive(state, tokens, go_work_label.name, line_no)
            else:
                fail("{}:{}: unexpected directive '{}'".format(go_work_label.name, line_no, current_directive))
        elif tokens[0] == "go":
            _validate_go_version(go_work_label.name, state, tokens, line_no)
            go = tokens[1]
        elif tokens[0] == "replace":
            if tokens[1] == "(":
                current_directive = tokens[0]
                continue
            else:
                _parse_replace_directive(state, tokens[1:], go_work_label.name, line_no)
        elif tokens[0] == "use":
            if len(tokens) != 2:
                fail("{}:{}: expected path or block in 'use' directive".format(go_work_label.name, line_no))
            elif tokens[1] == "(":
                current_directive = tokens[0]
                continue
            else:
                state["use"].append(tokens[1])
        elif tokens[0] == "toolchain":
            continue
        else:
            fail("{}:{}: unexpected directive '{}'".format(go_work_label.name, line_no, tokens[0]))

    major, minor = go.split(".")[:2]

    go_mods = [use_spec_to_label(go_work_label.repo_name, use) for use in state["use"]]
    from_file_tags = [struct(go_mod = go_mod, _is_dev_dependency = False) for go_mod in go_mods]

    module_tags = [struct(version = mod.version, path = mod.to_path, _parent_label = go_work_label, local_path = mod.local_path, indirect = False) for mod in state["replace"].values()]

    return struct(
        go = (int(major), int(minor)),
        from_file_tags = from_file_tags,
        replace_map = state["replace"],
        module_tags = module_tags,
        use = state["use"],
    )

# this exists because we are unable to create a path object in unit tests, we
# must do this as a post-process step or we cannot unit test go_mod parsing
def _relativize_replace_paths(go_mod, go_mod_path):
    new_replace_map = {}

    for key in go_mod.replace_map:
        value = go_mod.replace_map[key]

        local_path = value.local_path

        if local_path:
            # drop the go.mod from the path, to get the directory
            directory = go_mod_path.dirname

            # now that we have the directory, we can use the use replace directive to get the full path
            local_path = str(directory.get_child(local_path))

        new_replace_map[key] = struct(
            from_version = value.from_version,
            to_path = value.to_path,
            version = value.version,
            local_path = local_path,
        )

    new_go_mod = {
        attr: getattr(go_mod, attr)
        for attr in dir(go_mod)
        if not type(getattr(go_mod, attr)) == "builtin_function_or_method"
    }

    new_go_mod["replace_map"] = new_replace_map
    return struct(**new_go_mod)

def deps_from_go_mod(module_ctx, go_mod_label):
    """Loads the entries from a go.mod file.

    Args:
        module_ctx: a https://bazel.build/rules/lib/module_ctx object passed
            from the MODULE.bazel call.
        go_mod_label: a Label for a `go.mod` file.

    Returns:
        a tuple (Go module path, deps, replace map), where deps is a list of structs representing
        `require` statements from the go.mod file.
    """
    _check_go_mod_name(go_mod_label.name)

    go_mod_path = module_ctx.path(go_mod_label)
    go_mod_content = module_ctx.read(go_mod_path)
    go_mod = parse_go_mod(go_mod_content, go_mod_path)
    go_mod = _relativize_replace_paths(go_mod, go_mod_path)

    if go_mod.go[0] != 1 or go_mod.go[1] < 17:
        # go.mod files only include entries for all transitive dependencies as
        # of Go 1.17.
        fail("go_deps.from_file requires a go.mod file generated by Go 1.17 or later. Fix {} with 'go mod tidy -go=1.17'.".format(go_mod_label))

    deps = []
    for require in go_mod.require:
        deps.append(struct(
            path = require.path,
            version = require.version,
            indirect = require.indirect,
            local_path = None,
            _parent_label = go_mod_label,
        ))

    return go_mod.module, deps, go_mod.replace_map, go_mod.module

def parse_go_mod(content, path):
    # See https://go.dev/ref/mod#go-mod-file.

    # Valid directive values understood by this parser never contain tabs or
    # carriage returns, so we can simplify the parsing below by canonicalizing
    # whitespace upfront.
    content = content.replace("\t", " ").replace("\r", " ")

    state = {
        "module": None,
        "go": None,
        "require": [],
        "replace": {},
    }

    current_directive = None
    for line_no, line in enumerate(content.splitlines(), 1):
        tokens, comment = _tokenize_line(line, path, line_no)
        if not tokens:
            continue

        if not current_directive:
            if tokens[0] not in ["module", "go", "require", "replace", "exclude", "retract", "toolchain", "tool"]:
                fail("{}:{}: unexpected token '{}' at start of line".format(path, line_no, tokens[0]))
            if len(tokens) == 1:
                fail("{}:{}: expected another token after '{}'".format(path, line_no, tokens[0]))

            # The 'go' directive only has a single-line form and is thus parsed
            # here rather than in _parse_directive.
            if tokens[0] == "go":
                _validate_go_version(path, state, tokens, line_no)
                state["go"] = tokens[1]

            if tokens[1] == "(":
                current_directive = tokens[0]
                if len(tokens) > 2:
                    fail("{}:{}: unexpected token '{}' after '('".format(path, line_no, tokens[2]))
                continue

            _parse_directive(state, tokens[0], tokens[1:], comment, path, line_no)

        elif tokens[0] == ")":
            current_directive = None
            if len(tokens) > 1:
                fail("{}:{}: unexpected token '{}' after ')'".format(path, line_no, tokens[1]))
            continue

        else:
            _parse_directive(state, current_directive, tokens, comment, path, line_no)

    module = state["module"]
    if not module:
        fail("Expected a module directive in go.mod file")

    go = state["go"]
    if not go:
        # "As of the Go 1.17 release, if the go directive is missing, go 1.16 is assumed."
        go = "1.16"

    # The go directive can contain patch and pre-release versions, but we omit them.
    major, minor = go.split(".")[:2]

    return struct(
        module = module,
        go = (int(major), int(minor)),
        require = tuple(state["require"]),
        replace_map = state["replace"],
    )

def _parse_directive(state, directive, tokens, comment, path, line_no):
    if directive == "module":
        if state["module"] != None:
            fail("{}:{}: unexpected second 'module' directive".format(path, line_no))
        if len(tokens) > 1:
            fail("{}:{}: unexpected token '{}' after '{}'".format(path, line_no, tokens[1]))
        state["module"] = tokens[0]
    elif directive == "require":
        if len(tokens) != 2:
            fail("{}:{}: expected module path and version in 'require' directive".format(path, line_no))
        state["require"].append(struct(
            path = tokens[0],
            version = tokens[1],
            indirect = comment == "indirect",
        ))
    elif directive == "replace":
        _parse_replace_directive(state, tokens, path, line_no)

    # TODO: Handle exclude.

def _parse_replace_directive(state, tokens, path, line_no):
    # replacements key off of the from_path
    from_path = tokens[0]

    # pattern: replace from_path => to_path to_version
    if len(tokens) == 4 and tokens[1] == "=>":
        state["replace"][from_path] = struct(
            from_version = None,
            to_path = tokens[2],
            local_path = None,
            version = _canonicalize_raw_version(tokens[3]),
        )

        # pattern: replace from_path from_version => to_path to_version
    elif len(tokens) == 5 and tokens[2] == "=>":
        state["replace"][from_path] = struct(
            from_version = _canonicalize_raw_version(tokens[1]),
            to_path = tokens[3],
            version = _canonicalize_raw_version(tokens[4]),
            local_path = None,
        )

        # pattern: replace from_path from_version => file_path
    elif len(tokens) == 4 and tokens[2] == "=>":
        state["replace"][from_path] = struct(
            from_version = _canonicalize_raw_version(tokens[1]),
            to_path = from_path,
            local_path = tokens[3],
            version = COMPARES_HIGHEST_SENTINEL,
        )

        # pattern: replace from_path => to_path
    elif len(tokens) == 3 and tokens[1] == "=>":
        state["replace"][from_path] = struct(
            from_version = None,
            to_path = from_path,
            local_path = tokens[2],
            version = COMPARES_HIGHEST_SENTINEL,
        )
    else:
        fail("{}:{}: unexpected tokens '{}'".format(path, line_no, tokens))

def _tokenize_line(line, path, line_no):
    tokens = []
    r = line
    for _ in range(len(line)):
        r = r.strip()
        if not r:
            break

        if r[0] == "`":
            end = r.find("`", 1)
            if end == -1:
                fail("{}:{}: unterminated raw string".format(path, line_no))

            tokens.append(r[1:end])
            r = r[end + 1:]

        elif r[0] == "\"":
            value = ""
            escaped = False
            found_end = False
            for pos in range(1, len(r)):
                c = r[pos]

                if escaped:
                    value += c
                    escaped = False
                    continue

                if c == "\\":
                    escaped = True
                    continue

                if c == "\"":
                    found_end = True
                    break

                value += c

            if not found_end:
                fail("{}:{}: unterminated interpreted string".format(path, line_no))

            tokens.append(value)
            r = r[pos + 1:]

        elif r.startswith("//"):
            # A comment always ends the current line
            return tokens, r[len("//"):].strip()

        else:
            token, _, r = r.partition(" ")
            tokens.append(token)

    return tokens, None

def sums_from_go_mod(module_ctx, go_mod_label):
    """Loads the entries from a go.sum file given a go.mod Label.

    Args:
        module_ctx: a https://bazel.build/rules/lib/module_ctx object
            passed from the MODULE.bazel call.
        go_mod_label: a Label for a `go.mod` file. This label is used
            to find the associated `go.sum` file.

    Returns:
        A Dict[(string, string) -> (string)] is retruned where each entry
        is defined by a Go Module's sum:
            (path, version) -> (sum)
    """
    _check_go_mod_name(go_mod_label.name)

    return parse_sumfile(module_ctx, go_mod_label, "go.sum")

def sums_from_go_work(module_ctx, go_work_label):
    """Loads the entries from a go.work.sum file given a go.work label.

    Args:
        module_ctx: a https://bazel.build/rules/lib/module_ctx object
            passed from the MODULE.bazel call.
        go_work_label: a Label for a `go.work` file. This label is used
            to find the associated `go.work.sum` file.

    Returns:
        A Dict[(string, string) -> (string)] is returned where each entry
        is defined by a Go Module's sum:
            (path, version) -> (sum)
    """
    _check_go_work_name(go_work_label.name)

    # next we need to test if the go.work.sum file exists, this is a little tricky so we use an indirect approach:

    # 1. convert go_work_label into a path
    go_work_path = module_ctx.path(go_work_label)

    # 2. use the go_work_path to create a path for the heisen go.work.sum file
    maybe_go_work_sum_path = go_work_path.dirname.get_child("go.work.sum")

    # 3. check for its existence
    if maybe_go_work_sum_path.exists:
        return parse_sumfile(module_ctx, go_work_label, "go.work.sum")
    else:
        # 4. if go.work.sum does not exist, we should watch it in case it appears in the future
        if hasattr(module_ctx, "watch"):
            # module_ctx.watch_tree is only available in bazel >= 7.1
            module_ctx.watch(maybe_go_work_sum_path)

        # 5. return an empty dict as no sum file was found
        return {}

def parse_sumfile(module_ctx, label, sumfile):
    # We go through a Label so that the module extension is restarted if the sumfile
    # changes. We have to use a canonical label as we may not have visibility
    # into the module that provides the sumfile
    sum_label = Label("@@{}//{}:{}".format(
        label.repo_name,
        label.package,
        sumfile,
    ))

    return parse_go_sum(module_ctx.read(sum_label))

def parse_go_sum(content):
    hashes = {}
    for line in content.splitlines():
        path, version, sum = line.split(" ")
        version = _canonicalize_raw_version(version)
        if not version.endswith("/go.mod"):
            hashes[(path, version)] = sum
    return hashes

def _check_go_mod_name(name):
    if name != "go.mod":
        fail("go_deps.from_file requires a 'go.mod' file, not '{}'".format(name))

def _check_go_work_name(name):
    if name != "go.work":
        fail("go_deps.from_file requires a 'go.work' file, not '{}'".format(name))

def _canonicalize_raw_version(raw_version):
    if raw_version.startswith("v"):
        return raw_version[1:]
    return raw_version
