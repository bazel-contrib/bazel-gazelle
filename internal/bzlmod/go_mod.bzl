# Copyright 2026 The Bazel Authors. All rights reserved.
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

"Helper functions for go_deps related to go.mod and go.sum files"

def go_sum_info(*, sum, go_sum_label = None):
    """
    Tracks a sum of a Go module's content

    Args:
        sum: the cryptographic sum, like "h1:...".
        go_sum_label: Label of the go.sum or go.work.sum file the sum came
            from. None if the sum came from a go_deps.module tag.

    Returns:
        A struct containing the arguments as fields.
    """
    return struct(
        sum = sum,
        go_sum_label = go_sum_label,
    )

def parse_go_sum(content, go_sum_label = None):
    """
    Parses sums of module content from a go.sum or go.work.sum file

    Lines with sums of go.mod files (with versions ending in "/go.mod") are
    skipped. We only need module content sums.

    Args:
        content: the content of the file.
        go_sum_label: Label of the file, stored in each go_sum_info.

    Returns:
        A list of ((importpath, version), go_sum_info) tuples, in file order.
    """
    entries = []
    for line in content.split("\n"):
        fields = [f for f in line.replace("\t", " ").split(" ") if f]
        if len(fields) != 3 or fields[1].endswith("/go.mod"):
            continue
        entries.append(((fields[0], fields[1]), go_sum_info(sum = fields[2], go_sum_label = go_sum_label)))
    return entries

def add_sums(module_ctx, sums, entries):
    """
    Adds sums to a dict, reporting conflicts with sums already there

    Args:
        module_ctx: the module context, used to report conflicts.
        sums: dict mapping (importpath, version) tuples to go_sum_info
            structs. Modified.
        entries: list of ((importpath, version), go_sum_info) tuples, as
            returned by parse_go_sum.
    """
    for key, info in entries:
        existing = sums.get(key)
        if existing == None:
            sums[key] = info
            continue
        if existing.sum == info.sum:
            continue
        importpath, version = key
        module_ctx.fail("""\
Conflicting sums for Go module {importpath}@{version}:
    {old_sum} from {old_source}
    {new_sum} from {new_source}
""".format(
            importpath = importpath,
            version = version,
            old_sum = existing.sum,
            old_source = _sum_source(existing),
            new_sum = info.sum,
            new_source = _sum_source(info),
        ))

def _sum_source(info):
    if info.go_sum_label == None:
        return "a go_deps.module tag"
    return str(info.go_sum_label)
