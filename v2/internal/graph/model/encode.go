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

package model

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

type jsonGraph struct {
	Nodes      []jsonNode       `json:"nodes"`
	Edges      []jsonEdge       `json:"edges"`
	Unresolved []jsonUnresolved `json:"unresolved,omitempty"`
}

type jsonNode struct {
	Key      NodeKey `json:"key"`
	Kind     string  `json:"kind"`
	RuleKind string  `json:"rule_kind,omitempty"`
	Lang     string  `json:"lang,omitempty"`
	Origin   string  `json:"origin,omitempty"`
}

type jsonEdge struct {
	From   NodeKey      `json:"from"`
	To     NodeKey      `json:"to"`
	Kind   EdgeKind     `json:"kind"`
	Attr   string       `json:"attr,omitempty"`
	Select string       `json:"select,omitempty"`
	Reason []ImportSpec `json:"reason,omitempty"`
	Key    string       `json:"key,omitempty"`
	Data   string       `json:"data,omitempty"`
	Origin string       `json:"origin,omitempty"`
}

type jsonUnresolved struct {
	From   NodeKey    `json:"from"`
	Import ImportSpec `json:"import"`
	Attr   string     `json:"attr,omitempty"`
	Select string     `json:"select,omitempty"`
	Error  string     `json:"error"`
}

func (g *Graph) toJSON(withOrigin bool) jsonGraph {
	origin := func(o Origin) string {
		if !withOrigin {
			return ""
		}
		return o.String()
	}
	jg := jsonGraph{Nodes: []jsonNode{}, Edges: []jsonEdge{}}
	for _, n := range g.nodes {
		jg.Nodes = append(jg.Nodes, jsonNode{Key: n.Key, Kind: n.Kind.String(), RuleKind: n.RuleKind, Lang: n.Lang, Origin: origin(n.Origin)})
	}
	for _, r := range g.edges {
		je := jsonEdge{From: r.from, To: r.to, Kind: r.kind, Origin: origin(r.origin)}
		switch e := r.e.(type) {
		case DependsOn:
			je.Attr, je.Select, je.Reason = e.Attr, e.Select, e.Reason
		case Contains:
			je.Attr = e.Attr
		case ExtensionEdge:
			je.Key, je.Data = e.Key, e.Data
		}
		jg.Edges = append(jg.Edges, je)
	}
	for _, k := range g.UnresolvedAll() {
		for _, u := range g.unresolved[k] {
			msg := ""
			if u.Err != nil {
				msg = u.Err.Error()
			}
			jg.Unresolved = append(jg.Unresolved, jsonUnresolved{From: k, Import: u.Import, Attr: u.Attr, Select: u.Select, Error: msg})
		}
	}
	return jg
}

// WriteJSON writes the graph as indented JSON with a stable layout: nodes
// sorted by key, edges sorted by (from, kind, to, discriminator). The output
// is intended for debugging and as a basis for an offline index format; its
// schema is experimental.
func (g *Graph) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(g.toJSON(true))
}

// fingerprint returns a SHA-256 digest of the graph's structure: nodes,
// kinds, edges, payloads, and unresolved imports. Origins are excluded, so a
// graph built from generated rules and a graph rebuilt from the build files
// those rules were written to have the same fingerprint.
func (g *Graph) fingerprint() [32]byte {
	data, err := json.Marshal(g.toJSON(false))
	if err != nil {
		panic(fmt.Sprintf("graph: marshaling fingerprint: %v", err))
	}
	return sha256.Sum256(data)
}

// WriteDOT writes the graph in Graphviz DOT format for debugging.
func (g *Graph) WriteDOT(w io.Writer) error {
	ew := &errWriter{w: w}
	ew.printf("digraph gazelle {\n")
	for _, n := range g.nodes {
		shape := "box"
		switch n.Kind {
		case KindExternal:
			shape = "box, style=dashed"
		case KindSourceFile, KindGeneratedFile:
			shape = "note"
		case KindImport:
			shape = "ellipse"
		}
		ew.printf("  %s [shape=%s];\n", strconv.Quote(string(n.Key)), shape)
	}
	for _, r := range g.edges {
		lbl := string(r.kind)
		switch e := r.e.(type) {
		case DependsOn:
			lbl = e.Attr
			if e.Select != "" {
				lbl += " if " + e.Select
			}
		case Contains:
			lbl += " " + e.Attr
		}
		ew.printf("  %s -> %s [label=%s];\n", strconv.Quote(string(r.from)), strconv.Quote(string(r.to)), strconv.Quote(lbl))
	}
	ew.printf("}\n")
	return ew.err
}

type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) printf(format string, args ...any) {
	if ew.err == nil {
		_, ew.err = fmt.Fprintf(ew.w, format, args...)
	}
}
