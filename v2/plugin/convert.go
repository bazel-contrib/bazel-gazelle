/* Copyright 2026 The Bazel Authors. All rights reserved.

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

package plugin

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

// fileToProtocol converts a build file to its wire representation.
// It returns nil if f is nil.
func fileToProtocol(f *rule.File) *protocol.File {
	if f == nil {
		return nil
	}
	pf := &protocol.File{
		Path: f.Path,
		Pkg:  f.Pkg,
	}
	for _, d := range f.Directives {
		pf.Directives = append(pf.Directives, protocol.Directive{Key: d.Key, Value: d.Value})
	}
	for _, l := range f.Loads {
		pf.Loads = append(pf.Loads, protocol.Load{Name: l.Name(), Symbols: l.Symbols()})
	}
	pf.Rules = rulesToProtocol(f.Rules)
	return pf
}

func rulesToProtocol(rs []*rule.Rule) []protocol.Rule {
	if len(rs) == 0 {
		return nil
	}
	prs := make([]protocol.Rule, len(rs))
	for i, r := range rs {
		prs[i] = ruleToProtocol(r)
	}
	return prs
}

// ruleToProtocol converts a rule to its wire representation. Attribute values
// that can be represented as JSON are placed in Attrs; others are formatted
// as Starlark and placed in AttrExprs.
func ruleToProtocol(r *rule.Rule) protocol.Rule {
	pr := protocol.Rule{
		Kind: r.Kind(),
		Name: r.Name(),
		Keep: r.ShouldKeep(),
	}
	for _, key := range r.AttrKeys() {
		if key == "name" {
			continue
		}
		expr := r.Attr(key)
		if v, ok := exprToJSON(expr); ok {
			if pr.Attrs == nil {
				pr.Attrs = make(map[string]any)
			}
			pr.Attrs[key] = v
		} else {
			if pr.AttrExprs == nil {
				pr.AttrExprs = make(map[string]string)
			}
			pr.AttrExprs[key] = bzl.FormatString(expr)
		}
	}
	return pr
}

// exprToJSON converts a Starlark expression to a value that can be marshaled
// as JSON. It returns false if the expression has no JSON equivalent.
func exprToJSON(expr bzl.Expr) (any, bool) {
	switch e := expr.(type) {
	case *bzl.StringExpr:
		return e.Value, true

	case *bzl.Ident:
		switch e.Name {
		case "True":
			return true, true
		case "False":
			return false, true
		}
		return nil, false

	case *bzl.LiteralExpr:
		return literalToJSON(e.Token, false)

	case *bzl.UnaryExpr:
		if lit, ok := e.X.(*bzl.LiteralExpr); ok && e.Op == "-" {
			return literalToJSON(lit.Token, true)
		}
		return nil, false

	case *bzl.ListExpr:
		list := make([]any, len(e.List))
		for i, elem := range e.List {
			v, ok := exprToJSON(elem)
			if !ok {
				return nil, false
			}
			list[i] = v
		}
		return list, true

	case *bzl.DictExpr:
		dict := make(map[string]any, len(e.List))
		for _, kv := range e.List {
			k, ok := kv.Key.(*bzl.StringExpr)
			if !ok {
				return nil, false
			}
			if _, dup := dict[k.Value]; dup {
				return nil, false
			}
			v, ok := exprToJSON(kv.Value)
			if !ok {
				return nil, false
			}
			dict[k.Value] = v
		}
		return dict, true

	default:
		return nil, false
	}
}

func literalToJSON(token string, negate bool) (any, bool) {
	if negate {
		token = "-" + token
	}
	if n, err := strconv.ParseInt(token, 0, 64); err == nil {
		return n, true
	}
	if f, err := strconv.ParseFloat(token, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return f, true
	}
	return nil, false
}

// jsonToValue converts a decoded JSON value (from [protocol.Unmarshal]) to a
// value accepted by [rule.Rule.SetAttr].
func jsonToValue(v any) (any, error) {
	switch v := v.(type) {
	case string, bool:
		return v, nil

	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n, nil
		}
		f, err := v.Float64()
		if err != nil {
			return nil, fmt.Errorf("invalid number %s", v)
		}
		return f, nil

	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1<<53 {
			return int64(v), nil
		}
		return v, nil

	case []any:
		allStrings := true
		for _, elem := range v {
			if _, ok := elem.(string); !ok {
				allStrings = false
				break
			}
		}
		if allStrings {
			strs := make([]string, len(v))
			for i, elem := range v {
				strs[i] = elem.(string)
			}
			return strs, nil
		}
		list := make([]any, len(v))
		for i, elem := range v {
			ev, err := jsonToValue(elem)
			if err != nil {
				return nil, err
			}
			list[i] = ev
		}
		return list, nil

	case map[string]any:
		// Build the dict expression directly so that values of mixed types are
		// supported. Keys are sorted for deterministic output.
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		dict := &bzl.DictExpr{ForceMultiLine: true}
		for _, k := range keys {
			ev, err := jsonToValue(v[k])
			if err != nil {
				return nil, err
			}
			dict.List = append(dict.List, &bzl.KeyValueExpr{
				Key:   &bzl.StringExpr{Value: k},
				Value: rule.ExprFromValue(ev),
			})
		}
		return dict, nil

	case nil:
		return nil, fmt.Errorf("null is not a valid attribute value; use deleteAttrs to remove an attribute")

	default:
		return nil, fmt.Errorf("unsupported attribute value of type %T", v)
	}
}

// parseExpr parses a Starlark expression.
func parseExpr(src string) (bzl.Expr, error) {
	f, err := bzl.ParseBuild("expr", []byte("_ = "+src+"\n"))
	if err != nil {
		return nil, fmt.Errorf("parsing expression %q: %v", src, err)
	}
	if len(f.Stmt) != 1 {
		return nil, fmt.Errorf("parsing expression %q: expected a single expression", src)
	}
	assign, ok := f.Stmt[0].(*bzl.AssignExpr)
	if !ok {
		return nil, fmt.Errorf("parsing expression %q: expected a single expression", src)
	}
	return assign.RHS, nil
}

// applyAttrEdit applies attribute changes from a plugin to a rule.
func applyAttrEdit(r *rule.Rule, e protocol.AttrEdit) error {
	var errs []string
	for _, key := range sortedKeys(e.Attrs) {
		if _, dup := e.AttrExprs[key]; dup {
			errs = append(errs, fmt.Sprintf("attribute %q set in both attrs and attrExprs", key))
			continue
		}
		v, err := jsonToValue(e.Attrs[key])
		if err != nil {
			errs = append(errs, fmt.Sprintf("attribute %q: %v", key, err))
			continue
		}
		if key == "name" {
			name, ok := v.(string)
			if !ok {
				errs = append(errs, "attribute \"name\" must be a string")
				continue
			}
			r.SetName(name)
			continue
		}
		r.SetAttr(key, v)
	}
	for _, key := range sortedKeys(e.AttrExprs) {
		if _, dup := e.Attrs[key]; dup {
			continue // reported above
		}
		expr, err := parseExpr(e.AttrExprs[key])
		if err != nil {
			errs = append(errs, fmt.Sprintf("attribute %q: %v", key, err))
			continue
		}
		r.SetAttr(key, expr)
	}
	for _, key := range e.DeleteAttrs {
		if key == "name" {
			errs = append(errs, "attribute \"name\" can't be deleted")
			continue
		}
		r.DelAttr(key)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// ruleFromProtocol creates a new rule from its wire representation.
func ruleFromProtocol(pr protocol.Rule) (*rule.Rule, error) {
	if pr.Kind == "" {
		return nil, fmt.Errorf("rule %q has no kind", pr.Name)
	}
	r := rule.NewRule(pr.Kind, pr.Name)
	if err := applyAttrEdit(r, protocol.AttrEdit{Attrs: pr.Attrs, AttrExprs: pr.AttrExprs}); err != nil {
		return nil, fmt.Errorf("rule %s %q: %v", pr.Kind, pr.Name, err)
	}
	return r, nil
}

// kindFromProtocol converts a plugin's kind description.
func kindFromProtocol(k protocol.KindInfo) (rule.KindInfo, error) {
	if k.Name == "" {
		return rule.KindInfo{}, fmt.Errorf("kind has no name")
	}
	ki := rule.KindInfo{
		Name:            k.Name,
		MatchAny:        k.MatchAny,
		MatchAttrs:      k.MatchAttrs,
		NonEmptyAttrs:   toSet(k.NonEmptyAttrs),
		SubstituteAttrs: toSet(k.SubstituteAttrs),
		MergeableAttrs:  toSet(k.MergeableAttrs),
		ResolveAttrs:    toSet(k.ResolveAttrs),
	}
	if k.LoadedFrom != "" {
		l, err := label.Parse(k.LoadedFrom)
		if err != nil {
			return rule.KindInfo{}, fmt.Errorf("kind %s: invalid loadedFrom label %q: %v", k.Name, k.LoadedFrom, err)
		}
		if l.Relative {
			return rule.KindInfo{}, fmt.Errorf("kind %s: loadedFrom label %q must be absolute", k.Name, k.LoadedFrom)
		}
		ki.LoadedFrom = l
	}
	return ki, nil
}

func toSet(keys []string) map[string]bool {
	if len(keys) == 0 {
		return nil
	}
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
