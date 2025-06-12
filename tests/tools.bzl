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

def get_binary(target):
    """Get proper binary for test."""
    if "fix_load_for_packed_rules" in target:
        return ":gazelle_with_language_load_for_packed_rules"

    if "loads_from_flag" in target:
        return ":gazelle_with_language_loads_from_flag"

    if "fix_package_proto_name_match" in target:
        return ":gazelle_with_proto_and_go_languages"

    if target.startswith("go_"):
        return ":gazelle_with_proto_and_go_languages"

    return ":gazelle"
