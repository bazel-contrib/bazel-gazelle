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

load("@bazel_features//:features.bzl", "bazel_features")

visibility([
    "//tests/bzlmod/...",
])

# structs have certain built-in methods or attributes that should not
# be overwritten. Load these by calling `dir` on an empty struct.
# e.g. ["to_proto", "to_json"]
_STRUCT_PROTECTED_ATTRIBUTES = dir(struct())

def drop_nones(dict):
    """Drop entries with None values from a dictionary.

    Args:
        dict: A dictionary.

    Returns:
        A new dictionary with the same keys as dict, but with entries with
        None values removed.
    """
    return {k: v for k, v in dict.items() if v != None}

def get_directive_value(directives, key):
    """Get the value of a directive.

    Args:
        directives: A list of directives.
        key: The key of the directive.

    Returns:
        The value of the last directive, or None if the directive is not
        present.
    """
    prefix = "gazelle:" + key + " "

    value = None
    for directive in directives:
        if directive.startswith(prefix):
            # Treat "gazelle:key    value" the same as "gazelle:key value".
            value = directive[len(prefix):].lstrip()

    return value

def with_replaced_or_new_fields(_struct, **replacements):
    """Provides a shallow copy of a structure with replacements and/or new fields

    Args:
        _struct: structure to shallow copy.
        **replacements: kwargs for fields to either replace or add to the new struct.

    Returns:
        The resulting updated structure.
    """

    new_struct_assignments = {
        key: getattr(_struct, key)
        for key in dir(_struct)
        if key not in _STRUCT_PROTECTED_ATTRIBUTES
    }

    # Overwrite existing fields and add new ones.
    for key, value in replacements.items():
        new_struct_assignments[key] = value

    return struct(**new_struct_assignments)

def extension_metadata(
        module_ctx,
        *,
        root_module_direct_deps = None,
        root_module_direct_dev_deps = None,
        reproducible = False):
    if not hasattr(module_ctx, "extension_metadata"):
        return None
    metadata_kwargs = {}
    if bazel_features.external_deps.extension_metadata_has_reproducible:
        metadata_kwargs["reproducible"] = reproducible
    return module_ctx.extension_metadata(
        root_module_direct_deps = root_module_direct_deps,
        root_module_direct_dev_deps = root_module_direct_dev_deps,
        **metadata_kwargs
    )
