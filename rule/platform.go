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

// Platform represents a GOOS/GOARCH pair. When Platform is used to describe
// sources, dependencies, or flags, either OS or Arch may be empty.
//
// DEPRECATED: do not use outside language/go. This type is Go-specific
// and should be moved to the Go extension.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.Platform instead.
//go:fix inline
type Platform = v2.Platform

// KnownPlatforms is the set of target platforms that Go supports. Gazelle
// will generate multi-platform build files using these tags. rules_go and
// Bazel may not actually support all of these.
//
// If updating this list, please run `bazel run @io_bazel_rules_go//go generate ./...`
//
// DEPRECATED: do not use outside language/go.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KnownPlatforms instead.
//go:fix inline
var KnownPlatforms = v2.KnownPlatforms

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.OSAliases instead.
//go:fix inline
var OSAliases = v2.OSAliases

// UnixOS is the set of GOOS values matched by the "unix" build tag.
// This list is from go/src/cmd/dist/build.go.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.UnixOS instead.
//go:fix inline
var UnixOS = v2.UnixOS

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KnownOSs instead.
//go:fix inline
var KnownOSs = v2.KnownOSs

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KnownOSSet instead.
//go:fix inline
var KnownOSSet = v2.KnownOSSet

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KnownArchs instead.
//go:fix inline
var KnownArchs = v2.KnownArchs

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KnownArchSet instead.
//go:fix inline
var KnownArchSet = v2.KnownArchSet

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KnownOSArchs instead.
//go:fix inline
var KnownOSArchs = v2.KnownOSArchs

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KnownArchOSs instead.
//go:fix inline
var KnownArchOSs = v2.KnownArchOSs
