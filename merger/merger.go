/* Copyright 2016 The Bazel Authors. All rights reserved.

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

// Package merger provides functions for merging generated rules into
// existing build files.
//
// Gazelle's normal workflow is roughly as follows:
//
// 1. Read metadata from sources.
//
// 2. Generate new rules.
//
// 3. Merge newly generated rules with rules in the existing build file
// if there is one.
//
// 4. Build an index of merged library rules for dependency resolution.
//
// 5. Resolve dependencies (i.e., convert import strings to deps labels).
//
// 6. Merge the newly resolved dependencies.
//
// 7. Write the merged file back to disk.
//
// This package is used for sets 3 and 6 above.
package merger

import (
	v2 "github.com/bazel-contrib/bazel-gazelle/v2/merger"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// Phase indicates which attributes should be merged in matching rules.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/merger.Phase instead.
//go:fix inline
type Phase = v2.Phase

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/merger.PreResolve instead.
//go:fix inline
const PreResolve = v2.PreResolve

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/merger.PostResolve instead.
//go:fix inline
const PostResolve = v2.PostResolve

// UnstableInsertIndexKey is the name of an internal attribute that may be set
// on newly generated rules. When MergeFile is given a generated rule that
// doesn't match any existing rule, MergeFile will insert the rule at the index
// indicated by this key instead of at the end of the file.
//
// This definition is unstable and may be removed in the future.
//
// TODO(jayconrod): make this stable *or* find a better way to express it.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/merger.UnstableInsertIndexKey instead.
//go:fix inline
const UnstableInsertIndexKey = v2.UnstableInsertIndexKey

// MergeFile combines information from newly generated rules with matching
// rules in an existing build file. MergeFile can also delete rules which
// are empty after merging.
//
// oldFile is the file to merge. It must not be nil.
//
// emptyRules is a list of stub rules (with no attributes other than name)
// which were not generated. These are merged with matching rules. The merged
// rules are deleted if they contain no attributes that make them buildable
// (e.g., srcs, deps, anything in rule.KindInfo.NonEmptyAttrs).
//
// genRules is a list of newly generated rules. These are merged with
// matching rules. A rule matches if it has the same kind and name or if
// some other attribute in rule.KindInfo.MatchAttrs matches (e.g.,
// "importpath" in go_library). Elements of genRules that don't match
// any existing rule are appended to the end of oldFile.
//
// phase indicates whether this is a pre- or post-resolve merge. Different
// attributes (rule.KindInfo.MergeableAttrs or ResolveAttrs) will be merged.
//
// kinds maps rule kinds (e.g., "go_library") to metadata that helps merge
// rules of that kind.
//
// When a generated and existing rule are merged, each attribute is merged
// separately. If an attribute is mergeable (according to KindInfo), values
// from the existing attribute are replaced by values from the generated
// attribute. Comments are preserved on values that are present in both
// versions of the attribute. If at attribute is not mergeable, the generated
// version of the attribute will be added if no existing attribute is present;
// otherwise, the existing attribute will be preserved.
//
// Note that "# keep" comments affect merging. If a value within an existing
// attribute is marked with a "# keep" comment, it will not be removed.
// If an attribute is marked with a "# keep" comment, it will not be merged.
// If a rule is marked with a "# keep" comment, the whole rule will not
// be modified.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/merger.MergeFile instead.
//go:fix inline
func MergeFile(oldFile *rule.File, emptyRules, genRules []*rule.Rule, phase Phase, kinds map[string]rule.KindInfo, aliasedKinds map[string]string) {
	v2.MergeFile(oldFile, emptyRules, genRules, phase, kinds, aliasedKinds)
}

// Match searches for a rule that can be merged with x in rules.
//
// A rule is considered a match if its kind is equal to x's kind AND either its
// name is equal OR at least one of the attributes in matchAttrs is equal.
//
// If there are no matches, nil and nil are returned.
//
// If a rule has the same name but a different kind, nill and an error
// are returned.
//
// If there is exactly one match, the rule and nil are returned.
//
// If there are multiple matches, match will attempt to disambiguate, based on
// the quality of the match (name match is best, then attribute match in the
// order that attributes are listed). If disambiguation is successful,
// the rule and nil are returned. Otherwise, nil and an error are returned.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/merger.Match instead.
//go:fix inline
func Match(rules []*rule.Rule, x *rule.Rule, info rule.KindInfo, aliasedKinds map[string]string) (*rule.Rule, error) {
	return v2.Match(rules, x, info, aliasedKinds)
}
