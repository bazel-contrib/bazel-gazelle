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
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bazel-contrib/bazel-gazelle/v2/cmd/gazelle/update"
)

func main() {
	log.SetPrefix("gazelle: ")
	log.SetFlags(0) // don't print timestamps

	// TODO(#2279): interpret arguments as paths relative to either
	// BUILD_WORKSPACE_DIRECTORY or BUILD_WORKING_DIRECTORY, depending on whether
	// the paths were specified as arguments to the gazelle macro or passed as
	// arguments to `bazel run`. Currently, we mix them together and interpret
	// relative to BUILD_WORKSPACE_DIRECTORY.
	var wd string
	if wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); wsDir != "" {
		wd = wsDir
	} else {
		var err error
		if wd, err = os.Getwd(); err != nil {
			fmt.Fprintf(os.Stderr, "gazelle: %v\n", err)
			os.Exit(1)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx, wd, os.Args[1:]); err != nil {
		if !errors.Is(err, update.ErrDiff) {
			fmt.Fprintf(os.Stderr, "gazelle: %v\n", err)
		}
		if !errors.Is(err, flag.ErrHelp) {
			os.Exit(1)
		}
	}
}

func run(ctx context.Context, wd string, args []string) error {
	return update.Run(ctx, languages, wd, args)
}
