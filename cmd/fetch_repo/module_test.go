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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFileLockAcrossProcesses(t *testing.T) {
	const processes = 32

	root := t.TempDir()
	lockPath := filepath.Join(root, "module.lock")
	tracePath := filepath.Join(root, "trace")
	start := make(chan struct{})
	errs := make(chan error, processes)
	var wg sync.WaitGroup
	for i := 0; i < processes; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			cmd := exec.Command(os.Args[0], "-test.run=^TestFileLockHelperProcess$")
			cmd.Env = append(os.Environ(),
				"GO_WANT_FETCH_REPO_LOCK_HELPER=1",
				"FETCH_REPO_LOCK_PATH="+lockPath,
				"FETCH_REPO_TRACE_PATH="+tracePath,
				fmt.Sprintf("FETCH_REPO_HELPER_ID=%d", i),
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("helper %d: %s: %w", i, out, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if t.Failed() {
		return
	}

	trace, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(trace)), "\n")
	if got, want := len(lines), processes*2; got != want {
		t.Fatalf("got %d trace lines, want %d:\n%s", got, want, trace)
	}
	for i := 0; i < len(lines); i += 2 {
		startFields := strings.Fields(lines[i])
		endFields := strings.Fields(lines[i+1])
		if len(startFields) != 2 || len(endFields) != 2 ||
			startFields[0] != endFields[0] || startFields[1] != "start" || endFields[1] != "end" {
			t.Fatalf("overlapping lock holders at trace lines %d-%d: %q, %q", i+1, i+2, lines[i], lines[i+1])
		}
	}
}

func TestFileLockHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_FETCH_REPO_LOCK_HELPER") != "1" {
		t.Skip("helper process")
	}

	lock, err := acquireFileLock(os.Getenv("FETCH_REPO_LOCK_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	trace, err := os.OpenFile(os.Getenv("FETCH_REPO_TRACE_PATH"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		t.Fatal(err)
	}
	defer trace.Close()
	id := os.Getenv("FETCH_REPO_HELPER_ID")
	if _, err := fmt.Fprintf(trace, "%s start\n", id); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := fmt.Fprintf(trace, "%s end\n", id); err != nil {
		t.Fatal(err)
	}
}
