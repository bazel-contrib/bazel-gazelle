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

// Package server implements the plugin side of Gazelle's subprocess plugin
// protocol in Go. It's useful for plugins that are written in Go but built
// and released separately from Gazelle. Plugins in other languages implement
// the protocol described in package
// [github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol] directly.
//
// A plugin implements [Plugin] and any of the optional interfaces in this
// package, then calls [Main] from its main function:
//
//	func main() {
//		server.Main(&myPlugin{})
//	}
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol"
)

// Plugin is the interface all plugins implement.
type Plugin interface {
	// Initialize is called once before any other method. It returns the
	// plugin's name, kinds, directives, and capabilities.
	Initialize(context.Context, protocol.InitializeParams) (protocol.InitializeResult, error)
}

// Configurer handles configure requests.
type Configurer interface {
	Configure(context.Context, protocol.ConfigureParams) (protocol.ConfigureResult, error)
}

// Fixer handles fix requests. Plugins must also set the fix capability.
type Fixer interface {
	Fix(context.Context, protocol.FixParams) (protocol.FixResult, error)
}

// Generator handles generate requests.
type Generator interface {
	Generate(context.Context, protocol.GenerateParams) (protocol.GenerateResult, error)
}

// Indexer handles imports requests.
type Indexer interface {
	Imports(context.Context, protocol.ImportsParams) (protocol.ImportsResult, error)
}

// Resolver handles resolve requests. Resolve may call [IndexFind].
type Resolver interface {
	Resolve(context.Context, protocol.ResolveParams) (protocol.ResolveResult, error)
}

// Finder handles find requests. Plugins must also set the find capability.
// Find may call [IndexFind].
type Finder interface {
	Find(context.Context, protocol.FindParams) (protocol.FindResult, error)
}

// LifecycleHandler handles onResolve and onFinish requests. Plugins must also
// set the lifecycle capability.
type LifecycleHandler interface {
	OnResolve(context.Context) error
	OnFinish(context.Context) error
}

// Main serves p on stdin and stdout until Gazelle closes stdin, then exits
// the process. If serving fails, Main prints the error to stderr and exits
// with a non-zero status.
func Main(p Plugin) {
	if err := Serve(context.Background(), p, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "plugin: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// Serve reads requests from r and writes responses to w until r reaches
// end of file. Requests are handled one at a time.
func Serve(ctx context.Context, p Plugin, r io.Reader, w io.Writer) error {
	conn := protocol.NewConn(r, w)
	ctx = context.WithValue(ctx, connKey{}, conn)
	for {
		msg, err := conn.ReadMessage()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		if !msg.IsRequest() {
			return fmt.Errorf("unexpected response with id %s", msg.ID)
		}
		result, err := dispatch(ctx, p, msg.Method, msg.Params)
		if len(msg.ID) == 0 {
			continue // notification
		}
		if err := conn.Reply(msg.ID, result, err); err != nil {
			return err
		}
	}
}

// IndexFind asks Gazelle to look up an import in its rule index. It may only
// be called with the context passed to Resolve or Find.
func IndexFind(ctx context.Context, params protocol.IndexFindParams) (protocol.IndexFindResult, error) {
	conn, ok := ctx.Value(connKey{}).(*protocol.Conn)
	if !ok {
		return protocol.IndexFindResult{}, errors.New("IndexFind called without a plugin request context")
	}
	var res protocol.IndexFindResult
	err := conn.Call(ctx, protocol.MethodIndexFind, params, &res, nil)
	return res, err
}

type connKey struct{}

func dispatch(ctx context.Context, p Plugin, method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MethodInitialize:
		return handle(ctx, raw, p.Initialize)

	case protocol.MethodConfigure:
		if h, ok := p.(Configurer); ok {
			return handle(ctx, raw, h.Configure)
		}
		return protocol.ConfigureResult{}, nil

	case protocol.MethodFix:
		if h, ok := p.(Fixer); ok {
			return handle(ctx, raw, h.Fix)
		}

	case protocol.MethodGenerate:
		if h, ok := p.(Generator); ok {
			return handle(ctx, raw, h.Generate)
		}
		return protocol.GenerateResult{}, nil

	case protocol.MethodImports:
		if h, ok := p.(Indexer); ok {
			return handle(ctx, raw, h.Imports)
		}
		return protocol.ImportsResult{}, nil

	case protocol.MethodResolve:
		if h, ok := p.(Resolver); ok {
			return handle(ctx, raw, h.Resolve)
		}
		return protocol.ResolveResult{}, nil

	case protocol.MethodFind:
		if h, ok := p.(Finder); ok {
			return handle(ctx, raw, h.Find)
		}

	case protocol.MethodOnResolve:
		if h, ok := p.(LifecycleHandler); ok {
			return protocol.Empty{}, h.OnResolve(ctx)
		}
		return protocol.Empty{}, nil

	case protocol.MethodOnFinish:
		if h, ok := p.(LifecycleHandler); ok {
			return protocol.Empty{}, h.OnFinish(ctx)
		}
		return protocol.Empty{}, nil

	case protocol.MethodShutdown:
		return protocol.Empty{}, nil
	}
	return nil, &protocol.Error{Code: protocol.CodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", method)}
}

func handle[P, R any](ctx context.Context, raw json.RawMessage, f func(context.Context, P) (R, error)) (any, error) {
	var params P
	if err := protocol.Unmarshal(raw, &params); err != nil {
		return nil, &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
	}
	res, err := f(ctx, params)
	if err != nil {
		return nil, err
	}
	return res, nil
}
