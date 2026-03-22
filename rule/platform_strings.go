/* Copyright 2017 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rule

import v2 "github.com/bazel-contrib/bazel-gazelle/v2/rule"

// PlatformConstraint represents a constraint_setting target for a particular
// OS/arch combination.
//
// DEPRECATED: do not use outside language/go.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.PlatformConstraint instead.
//go:fix inline
type PlatformConstraint = v2.PlatformConstraint

// PlatformStrings contains a set of strings associated with a buildable
// target in a package. This is used to store source file names,
// import paths, and flags.
//
// Strings are stored in four sets: generic strings, OS-specific strings,
// arch-specific strings, and OS-and-arch-specific strings. A string may not
// be duplicated within a list or across sets; however, a string may appear
// in more than one list within a set (e.g., in "linux" and "windows" within
// the OS set). Strings within each list should be sorted, though this may
// not be relied upon.
//
// DEPRECATED: do not use outside language/go. This type is Go-specific and
// should be moved to the Go extension.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.PlatformStrings instead.
//go:fix inline
type PlatformStrings = v2.PlatformStrings
