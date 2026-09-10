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
load("//internal/bzlmod:goproxy.bzl", "is_pseudoversion")

_IS_PSEUDOVERSION_CASES = [
    ("v1.0.0-20260102030405-abcdef123456", True),
    ("v1.2.3-0.20260102030405-abcdef123456", True),
    ("v1.2.3-0.20260102030405-abcdef123456+incompatible", True),
    ("v1.2.3-pre.0.20260102030405-abcdef123456", True),
    ("v1.2.3-pre.0.20260102030405-abcdef123456+incompatible", True),
    ("v1.2.3", False),
    ("v1.2.3-pre", False),
    ("v1.2.3-20260102030405", False),
    ("v1.2.3-20260102030405-pre", False),
]

def _is_pseudoversion_test_impl(ctx):
    env = unittest.begin(ctx)
    for v, want in _IS_PSEUDOVERSION_CASES:
        asserts.equals(env, want, is_pseudoversion(v), v)
    return unittest.end(env)

is_pseudoversion_test = unittest.make(_is_pseudoversion_test_impl)

def goproxy_test_suite(name):
    unittest.suite(
        name,
        is_pseudoversion_test,
    )
