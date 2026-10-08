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

// Package plugin lets Gazelle use language extensions that run as separate
// processes. Such plugins can be written in any language: Gazelle starts the
// plugin executable and exchanges JSON-RPC messages with it over stdin and
// stdout. See package [github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol]
// for the protocol, and package
// [github.com/bazel-contrib/bazel-gazelle/v2/plugin/server] for a helper that
// implements the plugin side in Go.
//
// Users register plugins with the -plugin command-line flag or with a
// "# gazelle:plugin" directive in the repository root build file; see
// [Loader]. Each plugin is adapted by [Language] to the extension interfaces
// in packages [github.com/bazel-contrib/bazel-gazelle/v2/language],
// [github.com/bazel-contrib/bazel-gazelle/v2/config], and
// [github.com/bazel-contrib/bazel-gazelle/v2/resolve].
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	gzerrors "github.com/bazel-contrib/bazel-gazelle/v2/errors"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

// shutdownTimeout is how long Close waits for a plugin to exit before
// killing it.
const shutdownTimeout = 5 * time.Second

// Options describes how to start a plugin process.
type Options struct {
	// Path is the path to the plugin executable.
	Path string

	// Args are additional command-line arguments for the plugin.
	Args []string

	// Env is the plugin's environment. If nil, Gazelle's environment is used.
	Env []string

	// Dir is the plugin's working directory. If empty, the repository root
	// is used.
	Dir string

	// Stderr receives the plugin's standard error output. If nil, it is
	// written to Gazelle's standard error.
	Stderr io.Writer
}

// Language is a Gazelle extension implemented by a plugin process.
//
// Language implements [language.Language], [language.Generator],
// [language.Fixer], [language.OnResolver], [language.OnFinisher],
// [config.Configurer], [resolve.Indexer], [resolve.Resolver], and
// [resolve.Finder]. Calls are forwarded to the plugin process one at a time.
type Language struct {
	path  string
	cmd   *exec.Cmd
	stdin io.WriteCloser
	conn  *protocol.Conn

	name            string
	kinds           []rule.KindInfo
	knownDirectives []string
	caps            protocol.Capabilities

	// mu serializes access to the connection and the fields below.
	mu sync.Mutex

	// busy is set while a request is in flight. It's read without holding mu
	// to detect re-entrant calls (see Find).
	busy atomic.Bool

	// failed is set when the plugin process exits unexpectedly or breaks the
	// protocol. The call that fails returns it as a critical error; later
	// calls do nothing.
	failed error

	// waited is set after cmd.Wait has been called; exitErr is its result.
	waited  bool
	exitErr error

	closed bool

	// callback holds state needed to answer index/find requests from the
	// plugin. It's only set while a resolve or find request is in flight.
	callback *callbackState
}

type callbackState struct {
	ctx    context.Context
	config *config.Config
	index  *resolve.RuleIndex

	// from is the label of the rule being resolved. hasFrom is false during
	// find requests.
	from    label.Label
	hasFrom bool
}

var (
	_ language.Language   = (*Language)(nil)
	_ language.Generator  = (*Language)(nil)
	_ language.Fixer      = (*Language)(nil)
	_ language.OnResolver = (*Language)(nil)
	_ language.OnFinisher = (*Language)(nil)
	_ config.Configurer   = (*Language)(nil)
	_ resolve.Indexer     = (*Language)(nil)
	_ resolve.Resolver    = (*Language)(nil)
	_ resolve.Finder      = (*Language)(nil)
)

var validName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// Start starts a plugin process and initializes it with the repository-wide
// configuration in c. c must have been initialized from command-line flags.
// The caller must call Close when the plugin is no longer needed.
func Start(ctx context.Context, c *config.Config, opts Options) (_ *Language, err error) {
	cmd := exec.CommandContext(ctx, opts.Path, opts.Args...)
	cmd.Dir = opts.Dir
	if cmd.Dir == "" {
		cmd.Dir = c.RepoRoot
	}
	cmd.Env = opts.Env
	cmd.Stderr = opts.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("plugin %s: %w", opts.Path, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("plugin %s: %w", opts.Path, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("plugin %s: %w", opts.Path, err)
	}
	l := &Language{
		path:  opts.Path,
		cmd:   cmd,
		stdin: stdin,
		conn:  protocol.NewConn(stdout, stdin),
		name:  opts.Path,
	}
	defer func() {
		if err != nil {
			l.kill()
		}
	}()

	params := protocol.InitializeParams{
		ProtocolVersion:     protocol.Version,
		RepoRoot:            c.RepoRoot,
		RepoName:            c.RepoName,
		WorkDir:             c.WorkDir,
		ValidBuildFileNames: nonNil(c.ValidBuildFileNames),
		IndexLibraries:      c.IndexLibraries,
		IndexLazy:           c.IndexLazy,
		ShouldFix:           c.ShouldFix,
		Strict:              c.Strict,
		Bzlmod:              c.Bzlmod,
	}
	var res protocol.InitializeResult
	if err := l.call(ctx, protocol.MethodInitialize, &params, &res, nil); err != nil {
		return nil, fmt.Errorf("plugin %s: initialize: %w", opts.Path, err)
	}
	if res.ProtocolVersion < 1 || res.ProtocolVersion > protocol.Version {
		return nil, fmt.Errorf("plugin %s: unsupported protocol version %d (this version of Gazelle supports versions 1 through %d)", opts.Path, res.ProtocolVersion, protocol.Version)
	}
	if !validName.MatchString(res.Name) {
		return nil, fmt.Errorf("plugin %s: invalid language name %q", opts.Path, res.Name)
	}
	l.name = res.Name
	switch res.Capabilities.Configure {
	case "":
		res.Capabilities.Configure = protocol.ConfigureAll
	case protocol.ConfigureAll, protocol.ConfigureBuildFiles, protocol.ConfigureDirectives, protocol.ConfigureNone:
	default:
		return nil, fmt.Errorf("plugin %s (%s): unknown configure mode %q", res.Name, opts.Path, res.Capabilities.Configure)
	}
	l.caps = res.Capabilities
	seenKinds := make(map[string]bool)
	for _, k := range res.Kinds {
		ki, err := kindFromProtocol(k)
		if err != nil {
			return nil, fmt.Errorf("plugin %s (%s): %w", res.Name, opts.Path, err)
		}
		if seenKinds[ki.Name] {
			return nil, fmt.Errorf("plugin %s (%s): kind %s declared more than once", res.Name, opts.Path, ki.Name)
		}
		seenKinds[ki.Name] = true
		l.kinds = append(l.kinds, ki)
	}
	l.knownDirectives = slices.Clone(res.KnownDirectives)
	return l, nil
}

// Name returns the language name reported by the plugin.
func (l *Language) Name() string { return l.name }

// Path returns the path to the plugin executable.
func (l *Language) Path() string { return l.path }

// Capabilities returns the optional features the plugin supports.
func (l *Language) Capabilities() protocol.Capabilities { return l.caps }

// Kinds returns the kinds of rules the plugin generates.
func (l *Language) Kinds() []rule.KindInfo { return l.kinds }

// KnownDirectives returns the directives the plugin interprets.
func (l *Language) KnownDirectives() []string { return l.knownDirectives }

// configKey is the key in config.Config.Exts where a plugin's configuration
// value is stored. It can't collide with an extension name, since names can't
// contain ':'.
func (l *Language) configKey() string { return "plugin:" + l.name }

// configValue returns the plugin's configuration value for a directory.
func (l *Language) configValue(c *config.Config) json.RawMessage {
	if c == nil {
		return nil
	}
	v, _ := c.Exts[l.configKey()].(json.RawMessage)
	return v
}

// Configure implements [config.Configurer].
func (l *Language) Configure(ctx context.Context, args config.ConfigureArgs) error {
	switch l.caps.Configure {
	case protocol.ConfigureNone:
		return nil
	case protocol.ConfigureBuildFiles:
		if args.File == nil {
			return nil
		}
	case protocol.ConfigureDirectives:
		if args.File == nil || !slices.ContainsFunc(args.File.Directives, func(d rule.Directive) bool {
			return slices.Contains(l.knownDirectives, d.Key)
		}) {
			return nil
		}
	}
	params := protocol.ConfigureParams{
		Config: l.configValue(args.Config),
		Rel:    args.Rel,
		File:   fileToProtocol(args.File),
	}
	var res protocol.ConfigureResult
	if err := l.call(ctx, protocol.MethodConfigure, &params, &res, nil); err != nil {
		return err
	}
	if len(res.Config) > 0 {
		args.Config.Exts[l.configKey()] = slices.Clone(res.Config)
	}
	return diagnosticsError(res.Errors)
}

// Fix implements [language.Fixer]. It is a no-op unless the plugin declared
// the fix capability.
func (l *Language) Fix(ctx context.Context, args language.FixArgs) error {
	if !l.caps.Fix || args.File == nil {
		return nil
	}
	params := protocol.FixParams{
		Config:    l.configValue(args.Config),
		Rel:       args.Rel,
		File:      *fileToProtocol(args.File),
		ShouldFix: args.Config.ShouldFix,
	}
	var res protocol.FixResult
	if err := l.call(ctx, protocol.MethodFix, &params, &res, nil); err != nil {
		return err
	}
	errs := []error{diagnosticsError(res.Errors)}
	rules := args.File.Rules
	for _, e := range res.Edits {
		if e.Index < 0 || e.Index >= len(rules) {
			errs = append(errs, fmt.Errorf("%s: fix: rule index %d out of range", args.File.Path, e.Index))
			continue
		}
		r := rules[e.Index]
		if e.Delete {
			r.Delete()
			continue
		}
		if e.Kind != "" {
			r.SetKind(e.Kind)
		}
		if e.Name != "" {
			r.SetName(e.Name)
		}
		if err := applyAttrEdit(r, e.AttrEdit); err != nil {
			errs = append(errs, fmt.Errorf("%s: fix: rule %s: %w", args.File.Path, r.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// Generate implements [language.Generator].
func (l *Language) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	params := protocol.GenerateParams{
		Config:       l.configValue(args.Config),
		Dir:          args.Dir,
		Rel:          args.Rel,
		File:         fileToProtocol(args.File),
		Subdirs:      nonNil(args.Subdirs),
		RegularFiles: nonNil(args.RegularFiles),
		GenFiles:     nonNil(args.GenFiles),
		OtherGen:     rulesToProtocol(args.OtherGen),
		OtherEmpty:   rulesToProtocol(args.OtherEmpty),
	}
	var res protocol.GenerateResult
	if err := l.call(ctx, protocol.MethodGenerate, &params, &res, nil); err != nil {
		return language.GenerateResult{}, err
	}
	errs := []error{diagnosticsError(res.Errors)}
	if len(res.Imports) > 0 && len(res.Imports) != len(res.Gen) {
		errs = append(errs, fmt.Errorf("%s: generate returned %d rules but %d imports", relOrRoot(args.Rel), len(res.Gen), len(res.Imports)))
		return language.GenerateResult{}, errors.Join(errs...)
	}
	result := language.GenerateResult{RelsToIndex: res.RelsToIndex}
	for i, pr := range res.Gen {
		r, err := ruleFromProtocol(pr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", relOrRoot(args.Rel), err))
			continue
		}
		result.Gen = append(result.Gen, r)
		if len(res.Imports) > 0 {
			result.Imports = append(result.Imports, slices.Clone(res.Imports[i]))
		}
	}
	for _, pr := range res.Empty {
		r, err := ruleFromProtocol(pr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", relOrRoot(args.Rel), err))
			continue
		}
		result.Empty = append(result.Empty, r)
	}
	return result, errors.Join(errs...)
}

// Imports implements [resolve.Indexer].
func (l *Language) Imports(ctx context.Context, args resolve.ImportsArgs) (resolve.ImportsResult, error) {
	var rel string
	if args.File != nil {
		rel = args.File.Pkg
	}
	params := protocol.ImportsParams{
		Config: l.configValue(args.Config),
		Rel:    rel,
		Rule:   ruleToProtocol(args.Rule),
	}
	var res protocol.ImportsResult
	if err := l.call(ctx, protocol.MethodImports, &params, &res, nil); err != nil {
		return resolve.ImportsResult{}, err
	}
	errs := []error{diagnosticsError(res.Errors)}
	result := resolve.ImportsResult{NotImportable: res.NotImportable}
	for _, imp := range res.Imports {
		result.Imports = append(result.Imports, resolve.ImportSpec{Lang: imp.Lang, Imp: imp.Imp})
	}
	for _, s := range res.Embeds {
		lbl, err := label.Parse(s)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: rule %s: invalid embed label %q: %w", relOrRoot(rel), args.Rule.Name(), s, err))
			continue
		}
		result.Embeds = append(result.Embeds, lbl.Abs(args.Config.RepoName, rel))
	}
	return result, errors.Join(errs...)
}

// Resolve implements [resolve.Resolver]. While the plugin handles the
// request, it may look up imports in args.Index with index/find requests.
func (l *Language) Resolve(ctx context.Context, args resolve.ResolveArgs) error {
	imports, _ := args.Imports.(json.RawMessage)
	params := protocol.ResolveParams{
		Config:  l.configValue(args.Config),
		Rel:     args.From.Pkg,
		Rule:    ruleToProtocol(args.Rule),
		From:    args.From.String(),
		Imports: imports,
	}
	cb := &callbackState{
		ctx:     ctx,
		config:  args.Config,
		index:   args.Index,
		from:    args.From,
		hasFrom: true,
	}
	var res protocol.ResolveResult
	if err := l.call(ctx, protocol.MethodResolve, &params, &res, cb); err != nil {
		return err
	}
	errs := []error{diagnosticsError(res.Errors)}
	if err := applyAttrEdit(args.Rule, res.AttrEdit); err != nil {
		errs = append(errs, fmt.Errorf("resolving %s: %w", args.From, err))
	}
	return errors.Join(errs...)
}

// Find implements [resolve.Finder]. It is a no-op unless the plugin declared
// the find capability. Find is also a no-op when called while the plugin is
// handling another request (for example, when the plugin's own index/find
// request couldn't be satisfied by the rule index), since the plugin can't
// handle nested requests.
func (l *Language) Find(ctx context.Context, args resolve.FindArgs) ([]resolve.FindResult, error) {
	if !l.caps.Find || l.busy.Load() {
		return nil, nil
	}
	params := protocol.FindParams{
		Config: l.configValue(args.Config),
		Import: protocol.ImportSpec{Lang: args.Import.Lang, Imp: args.Import.Imp},
		Lang:   args.Lang,
	}
	cb := &callbackState{
		ctx:    ctx,
		config: args.Config,
		index:  args.Index,
	}
	var res protocol.FindResult
	if err := l.call(ctx, protocol.MethodFind, &params, &res, cb); err != nil {
		return nil, err
	}
	errs := []error{diagnosticsError(res.Errors)}
	var results []resolve.FindResult
	for _, m := range res.Results {
		fr, err := findResultFromProtocol(m)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		results = append(results, fr)
	}
	return results, errors.Join(errs...)
}

// OnResolve implements [language.OnResolver].
func (l *Language) OnResolve(ctx context.Context) error {
	if !l.caps.Lifecycle {
		return nil
	}
	return l.call(ctx, protocol.MethodOnResolve, nil, nil, nil)
}

// OnFinish implements [language.OnFinisher].
func (l *Language) OnFinish(ctx context.Context) error {
	if !l.caps.Lifecycle {
		return nil
	}
	return l.call(ctx, protocol.MethodOnFinish, nil, nil, nil)
}

// Close asks the plugin to shut down and waits for it to exit. If the plugin
// doesn't exit promptly, it's killed. Close returns an error if the plugin
// exits with a non-zero status after a clean shutdown.
func (l *Language) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	if l.waited {
		return nil
	}

	done := make(chan error, 1)
	failed := l.failed
	go func() {
		if failed == nil {
			// Ignore errors: the plugin may exit without answering.
			_ = l.conn.Call(context.Background(), protocol.MethodShutdown, nil, nil, nil)
		}
		l.stdin.Close()
		done <- l.cmd.Wait()
	}()
	var err error
	select {
	case err = <-done:
	case <-time.After(shutdownTimeout):
		_ = l.cmd.Process.Kill()
		err = <-done
		err = fmt.Errorf("did not exit within %v after shutdown; killed: %w", shutdownTimeout, err)
	}
	l.waited = true
	l.exitErr = err
	if err != nil && failed == nil {
		return fmt.Errorf("plugin %s (%s): %w", l.name, l.path, err)
	}
	return nil
}

// call sends a request to the plugin and waits for its response. cb, if not
// nil, enables index/find requests from the plugin while the request is in
// flight.
func (l *Language) call(ctx context.Context, method string, params, result any, cb *callbackState) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failed != nil {
		// The failure was already reported as a critical error, so Gazelle
		// will exit without writing files. Don't report it again for every
		// remaining directory and rule.
		return nil
	}
	if l.closed {
		return fmt.Errorf("plugin %s: %s called after Close", l.name, method)
	}
	l.busy.Store(true)
	l.callback = cb
	defer func() {
		l.callback = nil
		l.busy.Store(false)
	}()

	err := l.conn.Call(ctx, method, params, result, l.handleCallback)
	if err == nil {
		return nil
	}
	var rpcErr *protocol.Error
	if errors.As(err, &rpcErr) {
		return withSeverity(rpcErrorSeverity(rpcErr), fmt.Errorf("%s: %s", method, rpcErr.Message))
	}

	// The connection is broken: the plugin crashed or wrote something that
	// isn't a protocol message. Kill it and report its exit status, since
	// that's usually more informative than the I/O error.
	l.kill()
	msg := err.Error()
	if errors.Is(err, io.EOF) {
		msg = "plugin closed its output"
	}
	if l.exitErr != nil {
		msg = fmt.Sprintf("%s (%v)", msg, l.exitErr)
	}
	l.failed = gzerrors.SeverityErrorf(gzerrors.Critical, "plugin %s (%s) failed during %s: %s", l.name, l.path, method, msg)
	return l.failed
}

// handleCallback answers requests the plugin sends while Gazelle is waiting
// for a response.
func (l *Language) handleCallback(_ context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case protocol.MethodIndexFind:
		cb := l.callback
		if cb == nil || cb.index == nil {
			return nil, &protocol.Error{
				Code:    protocol.CodeInvalidRequest,
				Message: "index/find may only be called while handling resolve or find",
			}
		}
		var p protocol.IndexFindParams
		if err := protocol.Unmarshal(params, &p); err != nil {
			return nil, &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
		}
		lang := p.Lang
		if lang == "" {
			lang = l.name
		}
		imp := resolve.ImportSpec{Lang: p.Import.Lang, Imp: p.Import.Imp}
		var results []resolve.FindResult
		if dep, ok := resolve.FindRuleWithOverride(cb.config, imp, lang); ok {
			// gazelle:resolve and gazelle:resolve_regexp directives take
			// precedence over the index, as they do for built-in extensions.
			if dep.Relative {
				dep = dep.Abs(cb.config.RepoName, cb.from.Pkg)
			}
			results = []resolve.FindResult{{Label: dep}}
		} else {
			var err error
			results, err = cb.index.Find(cb.ctx, cb.config, imp, lang)
			if err != nil && len(results) == 0 {
				return nil, err
			}
		}
		res := protocol.IndexFindResult{}
		for _, r := range results {
			m := protocol.FindMatch{Label: r.Label.String()}
			for _, e := range r.Embeds {
				m.Embeds = append(m.Embeds, e.String())
			}
			if cb.hasFrom {
				m.RelLabel = r.Label.Rel(cb.from.Repo, cb.from.Pkg).String()
				m.SelfImport = r.IsSelfImport(cb.from)
			}
			res.Results = append(res.Results, m)
		}
		return res, nil

	default:
		return nil, &protocol.Error{
			Code:    protocol.CodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", method),
		}
	}
}

// kill stops the plugin process and records its exit status. mu must be held
// or the Language must not be shared yet.
func (l *Language) kill() {
	if l.waited {
		return
	}
	_ = l.stdin.Close()
	_ = l.cmd.Process.Kill()
	l.exitErr = l.cmd.Wait()
	l.waited = true
}

func findResultFromProtocol(m protocol.FindMatch) (resolve.FindResult, error) {
	lbl, err := label.Parse(m.Label)
	if err != nil {
		return resolve.FindResult{}, fmt.Errorf("find: invalid label %q: %w", m.Label, err)
	}
	fr := resolve.FindResult{Label: lbl}
	for _, s := range m.Embeds {
		e, err := label.Parse(s)
		if err != nil {
			return resolve.FindResult{}, fmt.Errorf("find: invalid embed label %q: %w", s, err)
		}
		fr.Embeds = append(fr.Embeds, e)
	}
	return fr, nil
}

// diagnosticsError converts diagnostics returned by a plugin into an error
// that Gazelle's error handler understands.
func diagnosticsError(diags []protocol.Diagnostic) error {
	var errs []error
	for _, d := range diags {
		errs = append(errs, withSeverity(parseSeverity(d.Severity), errors.New(d.Message)))
	}
	return errors.Join(errs...)
}

func rpcErrorSeverity(e *protocol.Error) gzerrors.Severity {
	if e.Data == nil {
		return gzerrors.Error
	}
	return parseSeverity(e.Data.Severity)
}

func parseSeverity(s string) gzerrors.Severity {
	switch strings.ToLower(s) {
	case "warning":
		return gzerrors.Warning
	case "critical":
		return gzerrors.Critical
	default:
		return gzerrors.Error
	}
}

func withSeverity(sev gzerrors.Severity, err error) error {
	if sev == gzerrors.Error {
		return err
	}
	return gzerrors.WithSeverity(sev, err)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func relOrRoot(rel string) string {
	if rel == "" {
		return "."
	}
	return rel
}
