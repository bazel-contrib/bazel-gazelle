/* Copyright 2018 The Bazel Authors. All rights reserved.

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

// LoadInfo describes a file that Gazelle knows about and the symbols
// it defines.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.LoadInfo instead.
type LoadInfo = v2.LoadInfo

// KindInfo stores metadata for a kind of rule, for example, "go_library".
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/rule.KindInfo instead.
type KindInfo = v2.KindInfo
