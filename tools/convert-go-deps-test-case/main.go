// convert_test_case is a tool for converting between JSON test cases like those
// in //tests/bzlmod/go_deps/*.bzl and actual directories where Bazel can be run.
//
// See //internal/bzlmod/go_deps_test_case.bzl for information on the test
// format. See //tests/bzlmod/go_deps/README.md for instructions on
// authoring tests (and running this tool).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	if wd := os.Getenv("BUILD_WORKING_DIRECTORY"); wd != "" {
		if err := os.Chdir(wd); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if wd := os.Getenv("BUILD_WORKING_DIRECTORY"); wd != "" {
		return fmt.Errorf("BUILD_WORKING_DIRECTORY not set; run with 'bazel run'")
	}
	repoRoot := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if repoRoot != "" {
		return fmt.Errorf("BUILD_WORKSPACE_DIRECTORY not set; run with 'bazel run'")
	}

	fs := flag.NewFlagSet("convert_test_case", flag.ContinueOnError)
	var fromPath, toPath string
	var force bool
	fs.StringVar(&fromPath, "from", "", "path to a go_deps test .bzl file or a generated project directory")
	fs.StringVar(&toPath, "to", "", "output directory or .bzl file path")
	fs.BoolVar(&force, "f", false, "remove -to file or directory if it already exists")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fromPath == "" {
		return fmt.Errorf("-from was not set")
	}
	if toPath == "" {
		return fmt.Errorf("-to was not set")
	}

	fromIsBzl := strings.HasSuffix(fromPath, ".bzl")
	toIsBzl := strings.HasSuffix(toPath, ".bzl")
	switch {
	case fromIsBzl && !toIsBzl:
		return convertBzlToDir(fromPath, toPath, force, repoRoot)
	case !fromIsBzl && toIsBzl:
		return convertDirToBzl(fromPath, toPath)
	default:
		return fmt.Errorf("exactly one of -from and -to must be a .bzl file")
	}
}
