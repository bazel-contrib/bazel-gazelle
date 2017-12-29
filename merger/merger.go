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

// Package merger provides methods for merging parsed BUILD files.
package merger

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	bf "github.com/bazelbuild/buildtools/build"
)

const keep = "keep" // marker in srcs or deps to tell gazelle to preserve.

// MergableAttrs is the set of attribute names for each kind of rule that
// may be merged. When an attribute is mergeable, a generated value may
// replace or augment an existing value. If an attribute is not mergeable,
// existing values are preserved. Generated non-mergeable attributes may
// still be added to a rule if there is no corresponding existing attribute.
type MergeableAttrs map[string]map[string]bool

var (
	// PreResolveAttrs is the set of attributes that should be merged before
	// dependency resolution, i.e., everything except deps.
	PreResolveAttrs MergeableAttrs

	// PostResolveAttrs is the set of attributes that should be merged after
	// dependency resolution, i.e., deps.
	PostResolveAttrs MergeableAttrs

	// RepoAttrs is the set of attributes that should be merged in repository
	// rules in WORKSPACE.
	RepoAttrs MergeableAttrs
)

func init() {
	goKinds := []string{
		"go_library",
		"go_binary",
		"go_test",
		"go_proto_library",
	}
	allKinds := append(goKinds, "proto_library")

	preResolveCommonAttrs := []string{"srcs"}
	preResolveGoAttrs := []string{
		"cgo",
		"clinkopts",
		"copts",
		"embed",
		"importpath",
	}
	preResolveGoProtoAttrs := []string{
		"compilers",
		"proto",
	}

	PreResolveAttrs = make(MergeableAttrs)
	for _, kind := range allKinds {
		PreResolveAttrs[kind] = make(map[string]bool)
		for _, attr := range preResolveCommonAttrs {
			PreResolveAttrs[kind][attr] = true
		}
	}
	for _, kind := range goKinds {
		for _, attr := range preResolveGoAttrs {
			PreResolveAttrs[kind][attr] = true
		}
	}
	for _, attr := range preResolveGoProtoAttrs {
		PreResolveAttrs["go_proto_library"][attr] = true
	}

	postResolveCommonAttrs := []string{
		"deps",
		config.GazelleImportsKey,
	}

	PostResolveAttrs = make(MergeableAttrs)
	for _, kind := range allKinds {
		PostResolveAttrs[kind] = make(map[string]bool)
		for _, attr := range postResolveCommonAttrs {
			PostResolveAttrs[kind][attr] = true
		}
	}

	repoKinds := []string{"go_repository"}
	repoCommonAttrs := []string{
		"commit",
		"importpath",
		"remote",
		"sha256",
		"strip_prefix",
		"tag",
		"type",
		"urls",
		"vcs",
	}
	RepoAttrs = make(MergeableAttrs)
	for _, kind := range repoKinds {
		RepoAttrs[kind] = make(map[string]bool)
		for _, attr := range repoCommonAttrs {
			RepoAttrs[kind][attr] = true
		}
	}
}

// MergeFile merges the rules in genRules with matching rules in oldFile and
// adds unmatched rules to the end of the merged file. MergeFile also merges
// rules in empty with matching rules in oldFile and deletes rules that
// are empty after merging. attrs is the set of attributes to merge. Attributes
// not in this set will be left alone if they already exist.
func MergeFile(genRules []bf.Expr, empty []bf.Expr, oldFile *bf.File, attrs MergeableAttrs) (mergedFile *bf.File, mergedRules []bf.Expr) {
	if oldFile == nil {
		return &bf.File{Stmt: genRules}, genRules
	}
	if shouldIgnore(oldFile) {
		return nil, nil
	}

	mergedFile = new(bf.File)
	*mergedFile = *oldFile
	mergedFile.Stmt = make([]bf.Expr, 0, len(oldFile.Stmt))
	for _, s := range oldFile.Stmt {
		if oldRule, ok := s.(*bf.CallExpr); ok {
			if genRule, _, ok := match(empty, oldRule); ok && genRule != nil {
				s = mergeRule(genRule, oldRule, attrs, oldFile.Path)
				if s == nil {
					// Deleted empty rule
					continue
				}
			}
		}
		mergedFile.Stmt = append(mergedFile.Stmt, s)
	}
	oldStmtCount := len(mergedFile.Stmt)

	for _, s := range genRules {
		genRule, ok := s.(*bf.CallExpr)
		if !ok {
			log.Panicf("got %v expected only CallExpr in genRules", s)
		}
		oldRule, i, ok := match(mergedFile.Stmt[:oldStmtCount], genRule)
		if oldRule == nil {
			mergedFile.Stmt = append(mergedFile.Stmt, genRule)
			mergedRules = append(mergedRules, genRule)
			continue
		} else if ok {
			merged := mergeRule(genRule, oldRule, attrs, oldFile.Path)
			mergedFile.Stmt[i] = merged
			mergedRules = append(mergedRules, mergedFile.Stmt[i])
		}
	}

	return mergedFile, mergedRules
}

// mergeRule combines information from gen and old and returns an updated rule.
// Both rules must be non-nil and must have the same kind and same name.
// attrs is the set of attributes which may be merged.
// If nil is returned, the rule should be deleted.
func mergeRule(gen, old *bf.CallExpr, attrs MergeableAttrs, filename string) bf.Expr {
	if old != nil && shouldKeep(old) {
		return old
	}

	genRule := bf.Rule{Call: gen}
	oldRule := bf.Rule{Call: old}
	merged := *old
	merged.List = nil
	mergedRule := bf.Rule{Call: &merged}

	// Copy unnamed arguments from the old rule without merging. The only rule
	// generated with unnamed arguments is go_prefix, which we currently
	// leave in place.
	// TODO: maybe gazelle should allow the prefix to be changed.
	for _, a := range old.List {
		if b, ok := a.(*bf.BinaryExpr); ok && b.Op == "=" {
			break
		}
		merged.List = append(merged.List, a)
	}

	// Merge attributes from the old rule. Preserve comments on old attributes.
	// Assume generated attributes have no comments.
	kind := oldRule.Kind()
	for _, k := range oldRule.AttrKeys() {
		oldAttr := oldRule.AttrDefn(k)
		if !attrs[kind][k] || shouldKeep(oldAttr) {
			merged.List = append(merged.List, oldAttr)
			continue
		}

		oldExpr := oldAttr.Y
		genExpr := genRule.Attr(k)
		mergedExpr, err := mergeExpr(genExpr, oldExpr)
		if err != nil {
			start, end := oldExpr.Span()
			log.Printf("%s:%d.%d-%d.%d: could not merge expression", filename, start.Line, start.LineRune, end.Line, end.LineRune)
			mergedExpr = oldExpr
		}
		if mergedExpr != nil {
			mergedAttr := *oldAttr
			mergedAttr.Y = mergedExpr
			merged.List = append(merged.List, &mergedAttr)
		}
	}

	// Merge attributes from genRule that we haven't processed already.
	for _, k := range genRule.AttrKeys() {
		if mergedRule.Attr(k) == nil {
			mergedRule.SetAttr(k, genRule.Attr(k))
		}
	}

	if isEmpty(&merged) {
		return nil
	}
	return &merged
}

// mergeExpr combines information from gen and old and returns an updated
// expression. The following kinds of expressions are recognized:
//
//   * nil
//   * strings (can only be merged with strings)
//   * lists of strings
//   * a call to select with a dict argument. The dict keys must be strings,
//     and the values must be lists of strings.
//   * a list of strings combined with a select call using +. The list must
//     be the left operand.
//
// An error is returned if the expressions can't be merged, for example
// because they are not in one of the above formats.
func mergeExpr(gen, old bf.Expr) (bf.Expr, error) {
	if shouldKeep(old) {
		return old, nil
	}
	if gen == nil && (old == nil || isScalar(old)) {
		return nil, nil
	}
	if isScalar(gen) {
		return gen, nil
	}

	genExprs, err := extractPlatformStringsExprs(gen)
	if err != nil {
		return nil, err
	}
	oldExprs, err := extractPlatformStringsExprs(old)
	if err != nil {
		return nil, err
	}
	mergedExprs, err := mergePlatformStringsExprs(genExprs, oldExprs)
	if err != nil {
		return nil, err
	}
	return makePlatformStringsExpr(mergedExprs), nil
}

// platformStringsExprs is a set of sub-expressions that match the structure
// of package.PlatformStrings. rules.Generator produces expressions that
// follow this structure for srcs, deps, and other attributes, so this matches
// all non-scalar expressions generated by Gazelle.
//
// The matched expression has the form:
//
// [] + select({}) + select({}) + select({})
//
// The four collections may appear in any order, and some or all of them may
// be omitted (all fields are nil for a nil expression).
type platformStringsExprs struct {
	generic            *bf.ListExpr
	os, arch, platform *bf.DictExpr
}

// extractPlatformStringsExprs matches an expression and attempts to extract
// sub-expressions in platformStringsExprs. The sub-expressions can then be
// merged with corresponding sub-expressions. Any field in the returned
// structure may be nil. An error is returned if the given expression does
// not follow the pattern described by platformStringsExprs.
func extractPlatformStringsExprs(expr bf.Expr) (platformStringsExprs, error) {
	var ps platformStringsExprs
	if expr == nil {
		return ps, nil
	}

	// Break the expression into a sequence of expressions combined with +.
	var parts []bf.Expr
	for {
		binop, ok := expr.(*bf.BinaryExpr)
		if !ok {
			parts = append(parts, expr)
			break
		}
		parts = append(parts, binop.Y)
		expr = binop.X
	}

	// Process each part. They may be in any order.
	for _, part := range parts {
		switch part := part.(type) {
		case *bf.ListExpr:
			if ps.generic != nil {
				return platformStringsExprs{}, fmt.Errorf("expression could not be matched: multiple list expressions")
			}
			ps.generic = part

		case *bf.CallExpr:
			x, ok := part.X.(*bf.LiteralExpr)
			if !ok || x.Token != "select" || len(part.List) != 1 {
				return platformStringsExprs{}, fmt.Errorf("expression could not be matched: callee other than select or wrong number of args")
			}
			arg, ok := part.List[0].(*bf.DictExpr)
			if !ok {
				return platformStringsExprs{}, fmt.Errorf("expression could not be matched: select argument not dict")
			}
			var dict **bf.DictExpr
			for _, item := range arg.List {
				kv := item.(*bf.KeyValueExpr) // parser guarantees this
				k, ok := kv.Key.(*bf.StringExpr)
				if !ok {
					return platformStringsExprs{}, fmt.Errorf("expression could not be matched: dict keys are not all strings")
				}
				if k.Value == "//conditions:default" {
					continue
				}
				key, err := resolve.ParseLabel(k.Value)
				if err != nil {
					return platformStringsExprs{}, fmt.Errorf("expression could not be matched: dict key is not label: %q", k.Value)
				}
				if config.KnownOSSet[key.Name] {
					dict = &ps.os
					break
				}
				if config.KnownArchSet[key.Name] {
					dict = &ps.arch
					break
				}
				osArch := strings.Split(key.Name, "_")
				if len(osArch) != 2 || !config.KnownOSSet[osArch[0]] || !config.KnownArchSet[osArch[1]] {
					return platformStringsExprs{}, fmt.Errorf("expression could not be matched: dict key contains unknown platform: %q", k.Value)
				}
				dict = &ps.platform
				break
			}
			if dict == nil {
				// We could not identify the dict because it's empty or only contains
				// //conditions:default. We'll call it the platform dict to avoid
				// dropping it.
				dict = &ps.platform
			}
			if *dict != nil {
				return platformStringsExprs{}, fmt.Errorf("expression could not be matched: multiple selects that are either os-specific, arch-specific, or platform-specific")
			}
			*dict = arg
		}
	}
	return ps, nil
}

// makePlatformStringsExpr constructs a single expression from the
// sub-expressions in ps.
func makePlatformStringsExpr(ps platformStringsExprs) bf.Expr {
	makeSelect := func(dict *bf.DictExpr) bf.Expr {
		return &bf.CallExpr{
			X:    &bf.LiteralExpr{Token: "select"},
			List: []bf.Expr{dict},
		}
	}
	forceMultiline := func(e bf.Expr) {
		switch e := e.(type) {
		case *bf.ListExpr:
			e.ForceMultiLine = true
		case *bf.CallExpr:
			e.List[0].(*bf.DictExpr).ForceMultiLine = true
		}
	}

	var parts []bf.Expr
	if ps.generic != nil {
		parts = append(parts, ps.generic)
	}
	if ps.os != nil {
		parts = append(parts, makeSelect(ps.os))
	}
	if ps.arch != nil {
		parts = append(parts, makeSelect(ps.arch))
	}
	if ps.platform != nil {
		parts = append(parts, makeSelect(ps.platform))
	}

	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		return parts[0]
	}
	expr := parts[0]
	forceMultiline(expr)
	for _, part := range parts[1:] {
		forceMultiline(part)
		expr = &bf.BinaryExpr{
			Op: "+",
			X:  expr,
			Y:  part,
		}
	}
	return expr
}

func mergePlatformStringsExprs(gen, old platformStringsExprs) (platformStringsExprs, error) {
	var ps platformStringsExprs
	var err error
	ps.generic = mergeList(gen.generic, old.generic)
	if ps.os, err = mergeDict(gen.os, old.os); err != nil {
		return platformStringsExprs{}, err
	}
	if ps.arch, err = mergeDict(gen.arch, old.arch); err != nil {
		return platformStringsExprs{}, err
	}
	if ps.platform, err = mergeDict(gen.platform, old.platform); err != nil {
		return platformStringsExprs{}, err
	}
	return ps, nil
}

func mergeList(gen, old *bf.ListExpr) *bf.ListExpr {
	if old == nil {
		return gen
	}
	if gen == nil {
		gen = &bf.ListExpr{List: []bf.Expr{}}
	}

	// Build a list of strings from the gen list and keep matching strings
	// in the old list. This preserves comments. Also keep anything with
	// a "# keep" comment, whether or not it's in the gen list.
	genSet := make(map[string]bool)
	for _, v := range gen.List {
		if s := stringValue(v); s != "" {
			genSet[s] = true
		}
	}

	var merged []bf.Expr
	kept := make(map[string]bool)
	keepComment := false
	for _, v := range old.List {
		s := stringValue(v)
		if keep := shouldKeep(v); keep || genSet[s] {
			keepComment = keepComment || keep
			merged = append(merged, v)
			if s != "" {
				kept[s] = true
			}
		}
	}

	// Add anything in the gen list that wasn't kept.
	for _, v := range gen.List {
		if s := stringValue(v); kept[s] {
			continue
		}
		merged = append(merged, v)
	}

	if len(merged) == 0 {
		return nil
	}
	return &bf.ListExpr{
		List:           merged,
		ForceMultiLine: gen.ForceMultiLine || old.ForceMultiLine || keepComment,
	}
}

func mergeDict(gen, old *bf.DictExpr) (*bf.DictExpr, error) {
	if old == nil {
		return gen, nil
	}
	if gen == nil {
		gen = &bf.DictExpr{List: []bf.Expr{}}
	}

	var entries []*dictEntry
	entryMap := make(map[string]*dictEntry)

	for _, kv := range old.List {
		k, v, err := dictEntryKeyValue(kv)
		if err != nil {
			return nil, err
		}
		if _, ok := entryMap[k]; ok {
			return nil, fmt.Errorf("old dict contains more than one case named %q", k)
		}
		e := &dictEntry{key: k, oldValue: v}
		entries = append(entries, e)
		entryMap[k] = e
	}

	for _, kv := range gen.List {
		k, v, err := dictEntryKeyValue(kv)
		if err != nil {
			return nil, err
		}
		e, ok := entryMap[k]
		if !ok {
			e = &dictEntry{key: k}
			entries = append(entries, e)
			entryMap[k] = e
		}
		e.genValue = v
	}

	keys := make([]string, 0, len(entries))
	haveDefault := false
	for _, e := range entries {
		e.mergedValue = mergeList(e.genValue, e.oldValue)
		if e.key == "//conditions:default" {
			// Keep the default case, even if it's empty.
			haveDefault = true
			if e.mergedValue == nil {
				e.mergedValue = &bf.ListExpr{}
			}
		} else if e.mergedValue != nil {
			keys = append(keys, e.key)
		}
	}
	if len(keys) == 0 && (!haveDefault || len(entryMap["//conditions:default"].mergedValue.List) == 0) {
		return nil, nil
	}
	sort.Strings(keys)
	// Always put the default case last.
	if haveDefault {
		keys = append(keys, "//conditions:default")
	}

	mergedEntries := make([]bf.Expr, len(keys))
	for i, k := range keys {
		e := entryMap[k]
		mergedEntries[i] = &bf.KeyValueExpr{
			Key:   &bf.StringExpr{Value: e.key},
			Value: e.mergedValue,
		}
	}

	return &bf.DictExpr{List: mergedEntries, ForceMultiLine: true}, nil
}

type dictEntry struct {
	key                             string
	oldValue, genValue, mergedValue *bf.ListExpr
}

func dictEntryKeyValue(e bf.Expr) (string, *bf.ListExpr, error) {
	kv, ok := e.(*bf.KeyValueExpr)
	if !ok {
		return "", nil, fmt.Errorf("dict entry was not a key-value pair: %#v", e)
	}
	k, ok := kv.Key.(*bf.StringExpr)
	if !ok {
		return "", nil, fmt.Errorf("dict key was not string: %#v", kv.Key)
	}
	v, ok := kv.Value.(*bf.ListExpr)
	if !ok {
		return "", nil, fmt.Errorf("dict value was not list: %#v", kv.Value)
	}
	return k.Value, v, nil
}

// shouldIgnore checks whether "gazelle:ignore" appears at the beginning of
// a comment before or after any top-level statement in the file.
func shouldIgnore(oldFile *bf.File) bool {
	directives := config.ParseDirectives(oldFile)
	for _, d := range directives {
		if d.Key == "ignore" {
			return true
		}
	}
	return false
}

// shouldKeep returns whether an expression from the original file should be
// preserved. This is true if it has a prefix or end-of-line comment "keep".
// Note that bf.Rewrite recognizes "keep sorted" comments which are different,
// so we don't recognize comments that only start with "keep".
func shouldKeep(e bf.Expr) bool {
	for _, c := range append(e.Comment().Before, e.Comment().Suffix...) {
		text := strings.TrimSpace(strings.TrimPrefix(c.Token, "#"))
		if text == keep {
			return true
		}
	}
	return false
}

// match looks for the matching CallExpr in stmts. It returns a matching rule
// (or nil), the index of the rule, and whether the match was complete.
//
// match scans CallExprs in stmts based on the "name" attribute and the kind.
// If "name" and kind both match, the rule, its index, and true are returned.
// If "name" matches but "kind" does not, the rule, its index, and false are
// returned. If no rule matches "name", nil, -1, and false are returned.
func match(stmts []bf.Expr, x *bf.CallExpr) (*bf.CallExpr, int, bool) {
	xr := bf.Rule{x}
	xname := xr.Name()
	xkind := xr.Kind()
	for i, s := range stmts {
		y, ok := s.(*bf.CallExpr)
		if !ok {
			continue
		}
		yr := bf.Rule{Call: y}
		yname := yr.Name()
		if xname != yname {
			continue
		}
		ykind := yr.Kind()
		return y, i, xkind == ykind
	}
	return nil, -1, false
}

func kind(c *bf.CallExpr) string {
	return (&bf.Rule{c}).Kind()
}

func name(c *bf.CallExpr) string {
	return (&bf.Rule{c}).Name()
}

func isEmpty(c *bf.CallExpr) bool {
	for _, arg := range c.List {
		kwarg, ok := arg.(*bf.BinaryExpr)
		if !ok || kwarg.Op != "=" {
			return false
		}
		key, ok := kwarg.X.(*bf.LiteralExpr)
		if !ok {
			return false
		}
		if key.Token != "name" && key.Token != "visibility" {
			return false
		}
	}
	return true
}

func isScalar(e bf.Expr) bool {
	switch e.(type) {
	case *bf.StringExpr, *bf.LiteralExpr:
		return true
	default:
		return false
	}
}

func stringValue(e bf.Expr) string {
	s, ok := e.(*bf.StringExpr)
	if !ok {
		return ""
	}
	return s.Value
}
