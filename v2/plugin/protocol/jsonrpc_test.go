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

package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// pipePair returns two connections wired to each other.
func pipePair() (a, b *Conn, closeAll func()) {
	ar, bw := io.Pipe()
	br, aw := io.Pipe()
	return NewConn(ar, aw), NewConn(br, bw), func() {
		ar.Close()
		aw.Close()
		br.Close()
		bw.Close()
	}
}

func TestCallWithCallback(t *testing.T) {
	gazelle, plugin, closeAll := pipePair()
	defer closeAll()

	// The plugin answers a resolve request after looking up an import with an
	// index/find request back to Gazelle.
	pluginErr := make(chan error, 1)
	go func() {
		pluginErr <- func() error {
			msg, err := plugin.ReadMessage()
			if err != nil {
				return err
			}
			if msg.Method != MethodResolve {
				return errors.New("unexpected method " + msg.Method)
			}
			var params ResolveParams
			if err := Unmarshal(msg.Params, &params); err != nil {
				return err
			}
			var found IndexFindResult
			if err := plugin.Call(context.Background(), MethodIndexFind, IndexFindParams{Import: ImportSpec{Lang: "sh", Imp: "lib/util.sh"}}, &found, nil); err != nil {
				return err
			}
			result := ResolveResult{AttrEdit: AttrEdit{Attrs: map[string]any{"deps": []string{found.Results[0].RelLabel}}}}
			return plugin.Reply(msg.ID, result, nil)
		}()
	}()

	handler := func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		if method != MethodIndexFind {
			return nil, &Error{Code: CodeMethodNotFound, Message: method}
		}
		var p IndexFindParams
		if err := Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return IndexFindResult{Results: []FindMatch{{Label: "//lib:util", RelLabel: "//lib:" + strings.TrimSuffix(strings.TrimPrefix(p.Import.Imp, "lib/"), ".sh")}}}, nil
	}
	var res ResolveResult
	if err := gazelle.Call(context.Background(), MethodResolve, ResolveParams{From: "//bin:run"}, &res, handler); err != nil {
		t.Fatal(err)
	}
	if err := <-pluginErr; err != nil {
		t.Fatal(err)
	}
	deps, _ := res.Attrs["deps"].([]any)
	if len(deps) != 1 || deps[0] != "//lib:util" {
		t.Errorf("got deps %#v; want [//lib:util]", res.Attrs["deps"])
	}
}

func TestCallError(t *testing.T) {
	gazelle, plugin, closeAll := pipePair()
	defer closeAll()
	go func() {
		msg, err := plugin.ReadMessage()
		if err != nil {
			return
		}
		plugin.Reply(msg.ID, nil, &Error{Code: 1, Message: "bad file", Data: &ErrorData{Severity: "warning"}})
	}()
	err := gazelle.Call(context.Background(), MethodGenerate, GenerateParams{}, &GenerateResult{}, nil)
	var rpcErr *Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("got error %v; want *Error", err)
	}
	if rpcErr.Message != "bad file" || rpcErr.Data == nil || rpcErr.Data.Severity != "warning" {
		t.Errorf("got %#v", rpcErr)
	}
}

func TestCallbackWithoutHandler(t *testing.T) {
	gazelle, plugin, closeAll := pipePair()
	defer closeAll()
	gotErr := make(chan error, 1)
	go func() {
		msg, err := plugin.ReadMessage()
		if err != nil {
			gotErr <- err
			return
		}
		gotErr <- plugin.Call(context.Background(), MethodIndexFind, IndexFindParams{}, nil, nil)
		plugin.Reply(msg.ID, nil, nil)
	}()
	if err := gazelle.Call(context.Background(), MethodResolve, ResolveParams{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	var rpcErr *Error
	if err := <-gotErr; !errors.As(err, &rpcErr) || rpcErr.Code != CodeMethodNotFound {
		t.Errorf("got error %v; want method not found", err)
	}
}

func TestReadMessage(t *testing.T) {
	long := strings.Repeat("x", 1<<20)
	input := "\n" + `{"jsonrpc":"2.0","id":1,"method":"generate","params":{"rel":"` + long + `"}}` + "\n\n" +
		`{"jsonrpc":"2.0","id":2,"result":{}}`
	c := NewConn(strings.NewReader(input), io.Discard)

	msg, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var p GenerateParams
	if err := Unmarshal(msg.Params, &p); err != nil {
		t.Fatal(err)
	}
	if msg.Method != MethodGenerate || len(p.Rel) != len(long) {
		t.Errorf("first message not read correctly")
	}

	// The last line has no trailing newline.
	msg, err = c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.IsRequest() || string(msg.ID) != "2" {
		t.Errorf("got %#v; want response with id 2", msg)
	}

	if _, err := c.ReadMessage(); !errors.Is(err, io.EOF) {
		t.Errorf("got error %v; want io.EOF", err)
	}
}

func TestReadMessageInvalid(t *testing.T) {
	c := NewConn(strings.NewReader("not json\n"), io.Discard)
	if _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteMessageOneLine(t *testing.T) {
	var buf bytes.Buffer
	c := NewConn(strings.NewReader(""), &buf)
	if err := c.Reply(json.RawMessage("7"), Rule{Kind: "sh_library", Attrs: map[string]any{"srcs": []string{"a\nb.sh"}}}, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Errorf("message is not a single line: %q", out)
	}
	if !strings.Contains(out, `"jsonrpc":"2.0"`) || !strings.Contains(out, `"id":7`) {
		t.Errorf("unexpected message: %s", out)
	}
}

func TestUnmarshalNumbers(t *testing.T) {
	var r Rule
	if err := Unmarshal(json.RawMessage(`{"kind":"k","attrs":{"size":9007199254740993,"ratio":1.5}}`), &r); err != nil {
		t.Fatal(err)
	}
	if n, ok := r.Attrs["size"].(json.Number); !ok || n.String() != "9007199254740993" {
		t.Errorf("size: got %#v; want exact json.Number", r.Attrs["size"])
	}
	if n, ok := r.Attrs["ratio"].(json.Number); !ok || n.String() != "1.5" {
		t.Errorf("ratio: got %#v", r.Attrs["ratio"])
	}
}
