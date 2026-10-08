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

// Package protocol defines the wire protocol between Gazelle and language
// plugins that run as separate processes.
//
// A plugin is an executable written in any language. Gazelle starts it once
// per run and talks to it over the plugin's stdin and stdout using JSON-RPC
// 2.0. Each message is a single line of UTF-8 JSON terminated by '\n'
// (JSON Lines framing). The plugin's stderr is passed through to the user.
//
// Gazelle sends the requests named by the Method* constants below and the
// plugin answers each one. While handling resolve or find, a plugin may send
// an [MethodIndexFind] request back to Gazelle to look up an import in the
// rule index; Gazelle answers it before the plugin answers the original
// request. Requests are never sent concurrently: Gazelle waits for each
// response before sending its next request.
//
// The types in this package are the schema. Fields use lowerCamelCase JSON
// names, which also match the JSON mapping of an equivalent proto3 schema.
// New optional fields may be added in later versions of the protocol;
// implementations must ignore fields they don't recognize.
package protocol

import "encoding/json"

// Version is the protocol version implemented by this package. Gazelle sends
// it in [InitializeParams.ProtocolVersion]; the plugin echoes the version it
// implements in [InitializeResult.ProtocolVersion].
const Version = 1

// Methods sent by Gazelle to a plugin.
const (
	// MethodInitialize is the first request. Params: [InitializeParams].
	// Result: [InitializeResult].
	MethodInitialize = "initialize"

	// MethodConfigure is sent for directories according to
	// [Capabilities.Configure]. Params: [ConfigureParams].
	// Result: [ConfigureResult].
	MethodConfigure = "configure"

	// MethodFix is sent for each existing build file Gazelle will update, if
	// [Capabilities.Fix] is set. Params: [FixParams]. Result: [FixResult].
	MethodFix = "fix"

	// MethodGenerate is sent for each directory Gazelle will update.
	// Params: [GenerateParams]. Result: [GenerateResult].
	MethodGenerate = "generate"

	// MethodImports is sent for each rule of one of the plugin's kinds that
	// Gazelle adds to the rule index. Params: [ImportsParams].
	// Result: [ImportsResult].
	MethodImports = "imports"

	// MethodResolve is sent for each rule the plugin generated, after the rule
	// index is complete. Params: [ResolveParams]. Result: [ResolveResult].
	MethodResolve = "resolve"

	// MethodFind is sent when an import can't be found in the rule index, if
	// [Capabilities.Find] is set. Params: [FindParams]. Result: [FindResult].
	MethodFind = "find"

	// MethodOnResolve is sent after rules have been generated in all
	// directories, before dependency resolution, if [Capabilities.Lifecycle]
	// is set. Params and result are empty objects.
	MethodOnResolve = "onResolve"

	// MethodOnFinish is sent after build files have been written, if
	// [Capabilities.Lifecycle] is set. Params and result are empty objects.
	MethodOnFinish = "onFinish"

	// MethodShutdown is the last request. After answering it, the plugin
	// should exit when its stdin is closed. Params and result are empty
	// objects. Plugins must also exit if stdin is closed without a shutdown
	// request.
	MethodShutdown = "shutdown"
)

// Methods sent by a plugin to Gazelle.
const (
	// MethodIndexFind looks up an import in Gazelle's rule index. It may only
	// be sent while the plugin is handling a resolve or find request. Gazelle
	// applies gazelle:resolve and gazelle:resolve_regexp directives first; if
	// none matches, it searches the index, then asks extensions that
	// implement find. Params: [IndexFindParams]. Result: [IndexFindResult].
	MethodIndexFind = "index/find"
)

// InitializeParams is sent with [MethodInitialize]. It describes the
// repository-wide configuration after command-line flags are parsed.
type InitializeParams struct {
	// ProtocolVersion is the highest protocol version Gazelle supports.
	ProtocolVersion int `json:"protocolVersion"`

	// RepoRoot is the absolute path to the repository root directory.
	RepoRoot string `json:"repoRoot"`

	// RepoName is the name of the repository, if known.
	RepoName string `json:"repoName,omitempty"`

	// WorkDir is the directory used to resolve relative paths on the command
	// line (usually BUILD_WORKSPACE_DIRECTORY).
	WorkDir string `json:"workDir"`

	// ValidBuildFileNames lists the build file names Gazelle recognizes.
	// The first is used for new files.
	ValidBuildFileNames []string `json:"validBuildFileNames"`

	// IndexLibraries is true if Gazelle builds a rule index for dependency
	// resolution. IndexLazy is true if only selected directories are indexed
	// (see [GenerateResult.RelsToIndex]).
	IndexLibraries bool `json:"indexLibraries"`
	IndexLazy      bool `json:"indexLazy"`

	// ShouldFix is true if destructive fixes are allowed (gazelle fix, or
	// gazelle update -fix).
	ShouldFix bool `json:"shouldFix"`

	// Strict is true if Gazelle exits non-zero after reporting errors.
	Strict bool `json:"strict"`

	// Bzlmod is true if the repository uses Bzlmod.
	Bzlmod bool `json:"bzlmod"`
}

// InitializeResult is the plugin's answer to [MethodInitialize].
type InitializeResult struct {
	// ProtocolVersion is the protocol version the plugin implements. It must
	// be between 1 and [InitializeParams.ProtocolVersion].
	ProtocolVersion int `json:"protocolVersion"`

	// Name is the name of the language, like "sh" or "python". It is used in
	// log messages, in the gazelle:lang directive and -lang flag, and to
	// associate rules with the plugin. It must not collide with another
	// extension's name.
	Name string `json:"name"`

	// Kinds lists the kinds of rules the plugin generates.
	Kinds []KindInfo `json:"kinds,omitempty"`

	// KnownDirectives lists the directive keys the plugin interprets.
	// Gazelle reports directives not known by any extension as errors.
	KnownDirectives []string `json:"knownDirectives,omitempty"`

	// Capabilities lists optional features the plugin supports.
	Capabilities Capabilities `json:"capabilities"`
}

// ConfigureMode selects the directories for which Gazelle sends
// [MethodConfigure].
type ConfigureMode string

const (
	// ConfigureAll sends configure for every directory Gazelle visits. This
	// is the default.
	ConfigureAll ConfigureMode = "all"

	// ConfigureBuildFiles sends configure only for directories that have a
	// build file.
	ConfigureBuildFiles ConfigureMode = "buildFiles"

	// ConfigureDirectives sends configure only for directories whose build
	// file contains at least one of the plugin's known directives.
	ConfigureDirectives ConfigureMode = "directives"

	// ConfigureNone never sends configure. The config value is always null.
	ConfigureNone ConfigureMode = "none"
)

// Capabilities lists optional protocol features a plugin supports.
type Capabilities struct {
	// Configure selects the directories configure is sent for. Plugins that
	// only read their own directives should use [ConfigureDirectives] to
	// avoid a round trip for every directory.
	Configure ConfigureMode `json:"configure,omitempty"`

	// Fix is true if the plugin handles [MethodFix].
	Fix bool `json:"fix,omitempty"`

	// Find is true if the plugin handles [MethodFind].
	Find bool `json:"find,omitempty"`

	// Lifecycle is true if the plugin handles [MethodOnResolve] and
	// [MethodOnFinish].
	Lifecycle bool `json:"lifecycle,omitempty"`
}

// KindInfo describes a kind of rule a plugin generates, and how Gazelle should
// match and merge rules of that kind with existing rules.
type KindInfo struct {
	// Name is the rule kind, like "sh_library".
	Name string `json:"name"`

	// LoadedFrom is the label of the .bzl file that defines the kind, using
	// the module name as the repository name, like
	// "@rules_shell//shell:sh_library.bzl". Gazelle adds load statements for
	// generated rules. Empty for native rules.
	LoadedFrom string `json:"loadedFrom,omitempty"`

	// MatchAny is true if a generated rule may match any existing rule of the
	// same kind when there is exactly one.
	MatchAny bool `json:"matchAny,omitempty"`

	// MatchAttrs lists attributes used to match generated and existing rules.
	MatchAttrs []string `json:"matchAttrs,omitempty"`

	// NonEmptyAttrs lists attributes that prevent an existing rule from being
	// deleted when it matches an empty rule.
	NonEmptyAttrs []string `json:"nonEmptyAttrs,omitempty"`

	// SubstituteAttrs lists attributes containing labels of other generated
	// rules that must be updated when those rules are renamed by matching.
	SubstituteAttrs []string `json:"substituteAttrs,omitempty"`

	// MergeableAttrs lists attributes Gazelle merges into existing rules
	// before dependency resolution.
	MergeableAttrs []string `json:"mergeableAttrs,omitempty"`

	// ResolveAttrs lists attributes Gazelle merges into existing rules after
	// dependency resolution.
	ResolveAttrs []string `json:"resolveAttrs,omitempty"`
}

// File is an existing build file.
type File struct {
	// Path is the absolute path to the file.
	Path string `json:"path"`

	// Pkg is the Bazel package name (the slash-separated directory path
	// relative to the repository root, "" for the root).
	Pkg string `json:"pkg"`

	// Directives lists the gazelle directives in the file, in order.
	Directives []Directive `json:"directives,omitempty"`

	// Loads lists the file's load statements.
	Loads []Load `json:"loads,omitempty"`

	// Rules lists the rules (and other top-level calls) in the file, in order.
	Rules []Rule `json:"rules,omitempty"`
}

// Directive is a "# gazelle:key value" comment.
type Directive struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Load is a load statement.
type Load struct {
	// Name is the label of the loaded .bzl file.
	Name string `json:"name"`

	// Symbols lists the names bound by the load statement.
	Symbols []string `json:"symbols,omitempty"`
}

// Rule is a rule (a top-level call) in a build file.
//
// Attribute values are encoded in one of two ways. Values that can be written
// as plain JSON go in Attrs: strings, booleans, integers, lists, and dicts with
// string keys (as JSON objects). Any other value, like a select() or glob()
// expression, goes in AttrExprs as Starlark source code. An attribute must not
// appear in both maps.
type Rule struct {
	// Kind is the name of the rule's function, like "sh_library".
	Kind string `json:"kind"`

	// Name is the value of the rule's name attribute. It may be empty for
	// calls like package().
	Name string `json:"name,omitempty"`

	// Attrs holds attributes with JSON-representable values.
	Attrs map[string]any `json:"attrs,omitempty"`

	// AttrExprs holds attributes whose values are Starlark expressions.
	AttrExprs map[string]string `json:"attrExprs,omitempty"`

	// Keep is true if the rule has a "# keep" comment. Only set by Gazelle.
	Keep bool `json:"keep,omitempty"`
}

// AttrEdit is a set of changes to a rule's attributes.
type AttrEdit struct {
	// Attrs sets attributes to JSON-representable values.
	Attrs map[string]any `json:"attrs,omitempty"`

	// AttrExprs sets attributes to Starlark expressions.
	AttrExprs map[string]string `json:"attrExprs,omitempty"`

	// DeleteAttrs removes attributes.
	DeleteAttrs []string `json:"deleteAttrs,omitempty"`
}

// Diagnostic is a problem reported by a plugin along with a result.
type Diagnostic struct {
	// Message describes the problem. Include the file path if relevant.
	Message string `json:"message"`

	// Severity is "error" (the default), "warning", or "critical". Gazelle
	// stops early after a critical error and exits non-zero after an error in
	// strict mode.
	Severity string `json:"severity,omitempty"`
}

// ConfigureParams is sent with [MethodConfigure]. Configure is sent for
// parent directories before their subdirectories.
type ConfigureParams struct {
	// Config is the plugin's configuration value for the parent directory,
	// as returned by an earlier configure call. It is null for the
	// repository root, or if no configure call has returned a value yet.
	Config json.RawMessage `json:"config"`

	// Rel is the slash-separated path to the directory, relative to the
	// repository root ("" for the root).
	Rel string `json:"rel"`

	// File is the directory's build file, or null if there is none.
	File *File `json:"file"`
}

// ConfigureResult is the plugin's answer to [MethodConfigure].
type ConfigureResult struct {
	// Config is the plugin's configuration value for the directory. It may be
	// any JSON value. Gazelle stores it and sends it back in later requests
	// for this directory, and as the parent configuration when configuring
	// subdirectories. If omitted, the parent's value is kept.
	Config json.RawMessage `json:"config,omitempty"`

	Errors []Diagnostic `json:"errors,omitempty"`
}

// FixParams is sent with [MethodFix].
type FixParams struct {
	// Config is the plugin's configuration value for the directory.
	Config json.RawMessage `json:"config"`

	// Rel is the slash-separated path to the directory, relative to the
	// repository root.
	Rel string `json:"rel"`

	// File is the directory's existing build file.
	File File `json:"file"`

	// ShouldFix is true if destructive changes (deleting or renaming rules)
	// are allowed.
	ShouldFix bool `json:"shouldFix"`
}

// FixResult is the plugin's answer to [MethodFix].
type FixResult struct {
	// Edits lists changes to existing rules.
	Edits []RuleEdit `json:"edits,omitempty"`

	Errors []Diagnostic `json:"errors,omitempty"`
}

// RuleEdit changes an existing rule.
type RuleEdit struct {
	// Index identifies the rule by its position in [File.Rules].
	Index int `json:"index"`

	// Delete removes the rule.
	Delete bool `json:"delete,omitempty"`

	// Kind, if set, renames the rule's kind.
	Kind string `json:"kind,omitempty"`

	// Name, if set, renames the rule.
	Name string `json:"name,omitempty"`

	AttrEdit
}

// GenerateParams is sent with [MethodGenerate].
type GenerateParams struct {
	// Config is the plugin's configuration value for the directory.
	Config json.RawMessage `json:"config"`

	// Dir is the absolute path to the directory.
	Dir string `json:"dir"`

	// Rel is the slash-separated path to the directory, relative to the
	// repository root ("" for the root). It is also the package name.
	Rel string `json:"rel"`

	// File is the directory's existing build file, or null.
	File *File `json:"file"`

	// Subdirs lists subdirectories, RegularFiles lists regular files, and
	// GenFiles lists files generated by rules in the existing build file.
	// Paths are relative to Dir. RegularFiles and Subdirs include entries from
	// subdirectories without build files when Gazelle is configured to
	// generate only in existing packages.
	Subdirs      []string `json:"subdirs"`
	RegularFiles []string `json:"regularFiles"`
	GenFiles     []string `json:"genFiles"`

	// OtherGen and OtherEmpty list rules generated in this directory by
	// extensions that ran before this plugin.
	OtherGen   []Rule `json:"otherGen,omitempty"`
	OtherEmpty []Rule `json:"otherEmpty,omitempty"`
}

// GenerateResult is the plugin's answer to [MethodGenerate].
type GenerateResult struct {
	// Gen lists generated rules. Gazelle merges them into the build file.
	Gen []Rule `json:"gen,omitempty"`

	// Empty lists rules that can't be built from the files in the directory.
	// Matching existing rules are deleted if they are left empty after
	// merging.
	Empty []Rule `json:"empty,omitempty"`

	// Imports, if set, has one element per rule in Gen. Each is an arbitrary
	// JSON value describing the rule's imports. Gazelle sends it back in
	// [ResolveParams.Imports].
	Imports []json.RawMessage `json:"imports,omitempty"`

	// RelsToIndex lists slash-separated repository-relative directories to
	// index for dependency resolution when lazy indexing is enabled.
	RelsToIndex []string `json:"relsToIndex,omitempty"`

	Errors []Diagnostic `json:"errors,omitempty"`
}

// ImportSpec is a string by which a rule may be imported in a given language.
type ImportSpec struct {
	Lang string `json:"lang"`
	Imp  string `json:"imp"`
}

// ImportsParams is sent with [MethodImports].
type ImportsParams struct {
	// Config is the plugin's configuration value for the directory containing
	// the rule.
	Config json.RawMessage `json:"config"`

	// Rel is the package containing the rule.
	Rel string `json:"rel"`

	// Rule is the rule to index.
	Rule Rule `json:"rule"`
}

// ImportsResult is the plugin's answer to [MethodImports].
type ImportsResult struct {
	// Imports lists the import strings for the rule.
	Imports []ImportSpec `json:"imports,omitempty"`

	// Embeds lists labels of rules this rule embeds.
	Embeds []string `json:"embeds,omitempty"`

	// NotImportable is true for rules that can't be imported, like tests.
	NotImportable bool `json:"notImportable,omitempty"`

	Errors []Diagnostic `json:"errors,omitempty"`
}

// ResolveParams is sent with [MethodResolve].
type ResolveParams struct {
	// Config is the plugin's configuration value for the rule's directory.
	Config json.RawMessage `json:"config"`

	// Rel is the package containing the rule.
	Rel string `json:"rel"`

	// Rule is the generated rule, after merging with the existing build file.
	Rule Rule `json:"rule"`

	// From is the rule's absolute label.
	From string `json:"from"`

	// Imports is the value returned for this rule in
	// [GenerateResult.Imports], or null.
	Imports json.RawMessage `json:"imports"`
}

// ResolveResult is the plugin's answer to [MethodResolve]. It describes
// changes to the rule's attributes, usually setting "deps".
type ResolveResult struct {
	AttrEdit

	Errors []Diagnostic `json:"errors,omitempty"`
}

// FindParams is sent with [MethodFind].
type FindParams struct {
	// Config is the plugin's configuration value for the directory where the
	// import was found.
	Config json.RawMessage `json:"config"`

	// Import is the import to find.
	Import ImportSpec `json:"import"`

	// Lang is the language of the source file containing the import.
	Lang string `json:"lang"`
}

// FindResult is the plugin's answer to [MethodFind].
type FindResult struct {
	Results []FindMatch `json:"results,omitempty"`

	Errors []Diagnostic `json:"errors,omitempty"`
}

// FindMatch is a rule that may be imported with a given import string.
type FindMatch struct {
	// Label is the absolute label of the rule.
	Label string `json:"label"`

	// Embeds lists labels of rules the matched rule embeds, transitively.
	Embeds []string `json:"embeds,omitempty"`

	// RelLabel is Label written relative to the rule being resolved, suitable
	// for use in its deps. Only set by Gazelle in [IndexFindResult] during
	// resolve.
	RelLabel string `json:"relLabel,omitempty"`

	// SelfImport is true if the match is the rule being resolved, or a rule it
	// embeds. Only set by Gazelle in [IndexFindResult] during resolve.
	SelfImport bool `json:"selfImport,omitempty"`
}

// IndexFindParams is sent by a plugin with [MethodIndexFind].
type IndexFindParams struct {
	// Import is the import to look up.
	Import ImportSpec `json:"import"`

	// Lang is the language of the source file containing the import. If
	// empty, the plugin's name is used.
	Lang string `json:"lang,omitempty"`
}

// IndexFindResult is Gazelle's answer to [MethodIndexFind].
type IndexFindResult struct {
	Results []FindMatch `json:"results,omitempty"`
}

// Empty is used as the params and result of requests that carry no data.
type Empty struct{}
