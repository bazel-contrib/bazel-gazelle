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

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//internal/bzlmod:go_repository_config.bzl", "repository_config_inputs")

def _inputs(paths, prefix):
    return [paths[i] for i in repository_config_inputs(paths).get(prefix, [])]

def _repo_config_test_impl(ctx):
    env = unittest.begin(ctx)
    paths = ["example.com/dep", "elsewhere.dev/other"]
    asserts.equals(env, ["example.com/dep"], _inputs(paths, "example.com/dep"))
    asserts.equals(env, ["example.com/dep"], _inputs(paths + ["example.com/unrelated"], "example.com/dep"))
    asserts.equals(env, ["example.com/dep", "example.com/dep/sub"], _inputs(paths + ["example.com/dep/sub"], "example.com/dep"))
    asserts.equals(env, ["example.com/dep", "example.com"], _inputs(paths + ["example.com"], "example.com/dep"))
    asserts.equals(env, [], _inputs(paths, "unknown.dev"))
    asserts.equals(env, ["unknown.dev/new"], _inputs(paths + ["unknown.dev/new"], "unknown.dev"))

    # Versionless aliases and their ancestor bindings affect minimal module compatibility.
    paths = ["example.com/dep/v2", "example.com/dep/v3", "example.com"]
    asserts.equals(env, paths, _inputs(paths, "example.com/dep"))
    asserts.equals(env, [], _inputs(["example.com/dep/v01/pkg"], "example.com/dep/pkg"))
    return unittest.end(env)

repo_config_test = unittest.make(_repo_config_test_impl)
