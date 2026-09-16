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

"""Matches Go module paths against glob patterns like GOPRIVATE and GONOPROXY."""

visibility([
    "//",
    "//tests/bzlmod/...",
])

def match_prefix_patterns(globs, target):
    """
    Reports whether a path prefix of target matches one of a list of globs

    This is how Go interprets the GOPRIVATE, GONOPROXY, and GONOSUMDB
    environment variables, each a comma-separated list of glob patterns
    written to match a leading prefix of a module path. A pattern with n path
    elements is matched against the first n path elements of target, so
    "example.com/private" matches "example.com/private/m" but not
    "example.com/privateer".

    Patterns use the syntax of Go's path.Match: '*' matches a sequence of
    characters within a path element, '?' matches a single character, and
    '[...]' matches a character class. Malformed patterns never match, again
    following path.Match. Space is not stripped from patterns, so
    "a.com, b.com" contains the pattern " b.com", which matches nothing.

    Args:
        globs: comma-separated list of glob patterns, like the value of
            GOPRIVATE.
        target: a Go module path.

    Returns:
        True if any of the patterns matches a prefix of target.
    """
    target_elems = target.split("/")
    for glob in globs.split(","):
        if not glob:
            continue
        glob_elems = glob.split("/")
        if len(glob_elems) > len(target_elems):
            # Not enough prefix elements in target.
            continue
        if _match_elems(glob_elems, target_elems[:len(glob_elems)]):
            return True
    return False

def _match_elems(glob_elems, target_elems):
    for i, glob_elem in enumerate(glob_elems):
        tokens = _tokenize(glob_elem)
        if tokens == None:
            # Malformed pattern.
            return False
        if not _match_tokens(tokens, target_elems[i]):
            return False
    return True

# Token kinds produced by _tokenize. Each matches at most one character.
_LITERAL = "literal"  # a single character, in the "chars" field
_ANY = "any"  # '?'
_STAR = "star"  # '*', the only token that may match more than one character
_CLASS = "class"  # '[...]', with "negated" and a list of "ranges"

def _tokenize(glob_elem):
    """
    Splits one path element of a glob pattern into a list of token structs

    Returns None if the pattern is malformed, for example if a character class
    is not terminated or a backslash escape is missing its character.
    """
    tokens = []
    i = 0
    n = len(glob_elem)

    # Starlark has no while loop, and each iteration consumes at least one
    # character, so n iterations are enough.
    for _ in range(n):
        if i >= n:
            break
        c = glob_elem[i]
        if c == "*":
            tokens.append(struct(kind = _STAR))
            i += 1
        elif c == "?":
            tokens.append(struct(kind = _ANY))
            i += 1
        elif c == "\\":
            if i + 1 >= n:
                return None
            tokens.append(struct(kind = _LITERAL, chars = glob_elem[i + 1]))
            i += 2
        elif c == "[":
            token, i = _tokenize_class(glob_elem, i)
            if token == None:
                return None
            tokens.append(token)
        else:
            tokens.append(struct(kind = _LITERAL, chars = c))
            i += 1
    return tokens

def _tokenize_class(glob_elem, start):
    """
    Parses a '[...]' character class starting at start

    Returns:
        A (token, index) pair, where index is the position after the class.
        The token is None if the class is malformed.
    """
    n = len(glob_elem)
    i = start + 1
    negated = False
    if i < n and glob_elem[i] == "^":
        negated = True
        i += 1
    ranges = []
    for _ in range(n):
        if i >= n:
            # Unterminated class.
            return None, i
        if glob_elem[i] == "]" and ranges:
            return struct(kind = _CLASS, negated = negated, ranges = ranges), i + 1
        lo, i = _class_char(glob_elem, i)
        if lo == None:
            return None, i
        hi = lo
        if i + 1 < n and glob_elem[i] == "-":
            hi, i = _class_char(glob_elem, i + 1)
            if hi == None:
                return None, i
        ranges.append((lo, hi))
    return None, i

def _class_char(glob_elem, i):
    """Returns a (character, index) pair for one character within a class."""
    if glob_elem[i] == "\\":
        if i + 1 >= len(glob_elem):
            return None, i
        return glob_elem[i + 1], i + 2
    return glob_elem[i], i + 1

def _match_tokens(tokens, s):
    """
    Reports whether tokens match all of s

    Implements the usual backtracking match: when a '*' fails to lead to an
    overall match, retry with the '*' consuming one more character.
    """
    ti, si = 0, 0
    star_ti, star_si = -1, -1

    # Each iteration either advances si, advances ti, or advances the number of
    # characters consumed by the most recent '*', so this bound is generous.
    for _ in range((len(tokens) + len(s) + 2) * (len(s) + 2)):
        if ti == len(tokens) and si == len(s):
            return True
        if ti < len(tokens) and tokens[ti].kind == _STAR:
            star_ti, star_si = ti, si
            ti += 1
            continue
        if ti < len(tokens) and si < len(s) and _match_token(tokens[ti], s[si]):
            ti += 1
            si += 1
            continue
        if star_ti >= 0 and star_si < len(s):
            star_si += 1
            si = star_si
            ti = star_ti + 1
            continue
        return False
    return False

def _match_token(token, c):
    if token.kind == _ANY:
        return True
    if token.kind == _LITERAL:
        return token.chars == c
    if token.kind == _CLASS:
        matched = False
        for lo, hi in token.ranges:
            if lo <= c and c <= hi:
                matched = True
                break
        return matched != token.negated
    return False
