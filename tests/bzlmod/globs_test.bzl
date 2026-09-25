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
load("//internal/bzlmod:globs.bzl", "match_prefix_patterns")

# Each case is a (globs, target, want) triple, matching the behavior of
# module.MatchPrefixPatterns in golang.org/x/mod.
_MATCH_PREFIX_PATTERNS_CASES = [
    # No patterns.
    ("", "example.com/m", False),
    (",", "example.com/m", False),

    # Whole path elements are matched, including trailing elements of target.
    ("example.com/m", "example.com/m", True),
    ("example.com", "example.com/m", True),
    ("example.com/m", "example.com/m/sub", True),
    ("example.com/m", "example.com", False),
    ("example.com/m", "example.com/mm", False),
    ("example.com/m", "other.com/m", False),

    # Any pattern in the list may match. Space is not stripped, so a pattern
    # written with surrounding space matches nothing, as in Go.
    (" example.com/m , other.com ", "other.com/m", False),
    ("a.com,b.com,c.com", "b.com/m", True),
    ("a.com,b.com,c.com", "d.com/m", False),

    # '*' matches within a path element only.
    ("*.example.com", "private.example.com/m", True),
    ("*.example.com", "example.com/m", False),
    ("example.com/*", "example.com/m", True),
    ("example.com/*", "example.com/m/sub", True),
    ("example.com/*", "example.com", False),
    ("example.com/*/sub", "example.com/m/sub", True),
    ("example.com/*/sub", "example.com/m/other", False),
    ("*", "example.com/m", True),
    ("ex*le.com", "example.com/m", True),
    ("e*x*a*m*p*l*e.com", "example.com/m", True),
    ("example.com/m*n*o", "example.com/mno", True),
    ("example.com/m*n*o", "example.com/mnop", False),

    # '?' matches a single character.
    ("example.com/?", "example.com/m", True),
    ("example.com/?", "example.com/mm", False),

    # Character classes, optionally negated or given as ranges.
    ("example.com/[mn]", "example.com/n", True),
    ("example.com/[mn]", "example.com/o", False),
    ("example.com/[a-z]bc", "example.com/abc", True),
    ("example.com/[a-z]bc", "example.com/Abc", False),
    ("example.com/[^a-z]bc", "example.com/Abc", True),
    ("example.com/[^a-z]bc", "example.com/abc", False),

    # Escaped metacharacters match themselves.
    ("example.com/m\\*", "example.com/m*", True),
    ("example.com/m\\*", "example.com/mn", False),

    # Malformed patterns never match, but don't affect other patterns.
    ("example.com/[a-z", "example.com/abc", False),
    ("example.com/m\\", "example.com/m", False),
    ("example.com/[a-z,other.com", "other.com/m", True),
]

def _match_prefix_patterns_test_impl(ctx):
    env = unittest.begin(ctx)

    for globs, target, want in _MATCH_PREFIX_PATTERNS_CASES:
        asserts.equals(
            env,
            want,
            match_prefix_patterns(globs, target),
            "match_prefix_patterns({}, {})".format(repr(globs), repr(target)),
        )

    return unittest.end(env)

match_prefix_patterns_test = unittest.make(_match_prefix_patterns_test_impl)

def globs_test_suite(name):
    unittest.suite(
        name,
        match_prefix_patterns_test,
    )
