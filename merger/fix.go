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

package merger

import (
	v2 "github.com/bazel-contrib/bazel-gazelle/v2/merger"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// FixLoads removes loads of unused go rules and adds loads of newly used rules.
// This should be called after FixFile and MergeFile, since symbols
// may be introduced that aren't loaded.
//
// This function calls File.Sync before processing loads.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/merger.FixLoads instead.
//go:fix inline
func FixLoads(f *rule.File, knownLoads []rule.LoadInfo) {
	v2.FixLoads(f, knownLoads)
}

// CheckGazelleLoaded searches the given WORKSPACE file for a repository named
// "bazel_gazelle". If no such repository is found *and* the repo is not
// declared with a directive *and* at least one load statement mentions
// the repository, a descriptive error will be returned.
//
// This should be called after modifications have been made to WORKSPACE
// (i.e., after FixLoads) before writing it to disk.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/merger.CheckGazelleLoaded instead.
//go:fix inline
func CheckGazelleLoaded(f *rule.File) error {
	return v2.CheckGazelleLoaded(f)
}
