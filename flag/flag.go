// Copyright 2017 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package flag provides some general-purpose types which satisfy the
// flag.Value interface.
package flag

import v2 "github.com/bazel-contrib/bazel-gazelle/v2/flag"

// MultiFlag collects repeated string flags into a slice.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/flag.MultiFlag instead.
//go:fix inline
type MultiFlag = v2.MultiFlag

// ExplicitFlag is a string flag that tracks whether it was set.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/flag.ExplicitFlag instead.
//go:fix inline
type ExplicitFlag = v2.ExplicitFlag

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/flag.AllowedStringFlag instead.
//go:fix inline
type AllowedStringFlag = v2.AllowedStringFlag
