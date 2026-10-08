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

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol"
)

type minimalPlugin struct{}

func (minimalPlugin) Initialize(ctx context.Context, p protocol.InitializeParams) (protocol.InitializeResult, error) {
	return protocol.InitializeResult{ProtocolVersion: protocol.Version, Name: "minimal:" + p.RepoRoot}, nil
}

func TestServe(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"repoRoot":"/repo"}}`,
		`{"jsonrpc":"2.0","method":"onResolve"}`, // notification: no response
		`{"jsonrpc":"2.0","id":2,"method":"generate","params":{"rel":""}}`,
		`{"jsonrpc":"2.0","id":3,"method":"fix","params":{}}`,
		`{"jsonrpc":"2.0","id":4,"method":"initialize","params":{"repoRoot":7}}`,
		`{"jsonrpc":"2.0","id":5,"method":"shutdown"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := Serve(context.Background(), minimalPlugin{}, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	var responses []protocol.Message
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var msg protocol.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("invalid response %q: %v", line, err)
		}
		responses = append(responses, msg)
	}
	if len(responses) != 5 {
		t.Fatalf("got %d responses; want 5:\n%s", len(responses), out.String())
	}

	var init protocol.InitializeResult
	if err := json.Unmarshal(responses[0].Result, &init); err != nil || init.Name != "minimal:/repo" {
		t.Errorf("initialize: got %s, %v", responses[0].Result, err)
	}
	if string(responses[1].ID) != "2" || responses[1].Error != nil {
		t.Errorf("generate: unimplemented optional methods should return an empty result, got %+v", responses[1])
	}
	if e := responses[2].Error; e == nil || e.Code != protocol.CodeMethodNotFound {
		t.Errorf("fix: got %+v; want method not found", responses[2])
	}
	if e := responses[3].Error; string(responses[3].ID) != "4" || e == nil || e.Code != protocol.CodeInvalidParams {
		t.Errorf("initialize with bad params: got %+v; want invalid params", responses[3])
	}
	if string(responses[4].ID) != "5" || responses[4].Error != nil {
		t.Errorf("shutdown: got %+v", responses[4])
	}
}

func TestIndexFindOutsideRequest(t *testing.T) {
	if _, err := IndexFind(context.Background(), protocol.IndexFindParams{}); err == nil {
		t.Fatal("expected error")
	}
}
