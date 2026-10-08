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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Message is a JSON-RPC 2.0 request, notification, or response.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// IsRequest returns true if the message is a request or notification
// (as opposed to a response).
func (m *Message) IsRequest() bool {
	return m.Method != ""
}

// JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Error is a JSON-RPC error object. It is returned by [Conn.Call] when the
// peer answers a request with an error.
type Error struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    *ErrorData `json:"data,omitempty"`
}

// ErrorData is the optional data member of an [Error].
type ErrorData struct {
	// Severity is "error" (the default), "warning", or "critical".
	Severity string `json:"severity,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// Handler handles a request received by a [Conn]. It returns a value to be
// marshaled as the result or an error. If the error is an [*Error], it is
// sent as is; other errors are sent with [CodeInternalError].
type Handler func(ctx context.Context, method string, params json.RawMessage) (any, error)

// Conn sends and receives JSON-RPC 2.0 messages framed as JSON Lines: one
// JSON value per line, terminated by '\n'. Conn is not safe for concurrent
// use; the protocol never has more than one outstanding request in each
// direction.
type Conn struct {
	r      *bufio.Reader
	w      *bufio.Writer
	nextID int64
}

// NewConn returns a Conn that reads messages from r and writes them to w.
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{
		r: bufio.NewReaderSize(r, 64*1024),
		w: bufio.NewWriterSize(w, 64*1024),
	}
}

// ReadMessage reads the next message. Blank lines are skipped. ReadMessage
// returns io.EOF if the peer closed the stream cleanly.
func (c *Conn) ReadMessage() (*Message, error) {
	for {
		line, err := c.r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) == 0 {
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil, io.EOF
				}
				return nil, err
			}
			continue
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		msg := &Message{}
		if jerr := json.Unmarshal(line, msg); jerr != nil {
			return nil, fmt.Errorf("parsing message: %w: %q", jerr, truncate(line))
		}
		return msg, nil
	}
}

// WriteMessage writes a message on its own line and flushes it.
func (c *Conn) WriteMessage(msg *Message) error {
	msg.JSONRPC = "2.0"
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := c.w.Write(data); err != nil {
		return err
	}
	return c.w.Flush()
}

// Call sends a request and waits for the response, which is unmarshaled into
// result (unless result is nil). Requests the peer sends while Call is
// waiting are passed to handler, and their responses are written before
// Call continues waiting. If handler is nil, such requests are answered with
// [CodeMethodNotFound].
func (c *Conn) Call(ctx context.Context, method string, params, result any, handler Handler) error {
	c.nextID++
	id := json.RawMessage(strconv.FormatInt(c.nextID, 10))
	if params == nil {
		params = Empty{}
	}
	rawParams, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshaling %s params: %w", method, err)
	}
	if err := c.WriteMessage(&Message{ID: id, Method: method, Params: rawParams}); err != nil {
		return err
	}
	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return err
		}
		if msg.IsRequest() {
			if err := c.serve(ctx, msg, handler); err != nil {
				return err
			}
			continue
		}
		if !sameID(msg.ID, id) {
			return fmt.Errorf("received response with id %s while waiting for response to %s request with id %s", msg.ID, method, id)
		}
		if msg.Error != nil {
			return msg.Error
		}
		if result == nil {
			return nil
		}
		if err := Unmarshal(msg.Result, result); err != nil {
			return fmt.Errorf("parsing %s result: %w", method, err)
		}
		return nil
	}
}

// serve handles one request or notification from the peer.
func (c *Conn) serve(ctx context.Context, msg *Message, handler Handler) error {
	var result any
	var err error
	if handler == nil {
		err = &Error{Code: CodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", msg.Method)}
	} else {
		result, err = handler(ctx, msg.Method, msg.Params)
	}
	if len(msg.ID) == 0 {
		// Notification: no response.
		return nil
	}
	return c.Reply(msg.ID, result, err)
}

// Reply sends the response to the request with the given id. If err is not
// nil, an error response is sent and result is ignored.
func (c *Conn) Reply(id json.RawMessage, result any, err error) error {
	resp := &Message{ID: id}
	if err != nil {
		var rpcErr *Error
		if !errors.As(err, &rpcErr) {
			rpcErr = &Error{Code: CodeInternalError, Message: err.Error()}
		}
		resp.Error = rpcErr
	} else {
		if result == nil {
			result = Empty{}
		}
		data, merr := json.Marshal(result)
		if merr != nil {
			resp.Error = &Error{Code: CodeInternalError, Message: fmt.Sprintf("marshaling result: %v", merr)}
		} else {
			resp.Result = data
		}
	}
	return c.WriteMessage(resp)
}

// Unmarshal decodes JSON data into v. Numbers stored in interface values
// (like [Rule.Attrs]) are decoded as [json.Number] so integers are preserved
// exactly. Empty data and null leave v unchanged.
func Unmarshal(data json.RawMessage, v any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(v)
}

func sameID(a, b json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
}

func truncate(b []byte) []byte {
	const max = 200
	if len(b) > max {
		return b[:max]
	}
	return b
}
