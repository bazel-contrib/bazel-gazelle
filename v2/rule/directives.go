/* Copyright 2017 The Bazel Authors. All rights reserved.

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

package rule

import (
	"bufio"
	"fmt"
	"os"
	"regexp"

	bzl "github.com/bazelbuild/buildtools/build"
)

// Directive is a key-value pair extracted from a top-level comment in
// a build file. Directives have the following format:
//
//	# gazelle:key value
//
// Keys may not contain spaces. Values may be empty and may contain spaces,
// but surrounding space is trimmed.
type Directive struct {
	Key, Value string
}

// TODO(jayconrod): annotation directives will apply to an individual rule.
// They must appear in the block of comments above that rule.

// ParseDirectives scans f for Gazelle directives. The full list of directives
// is returned. Errors are reported for unrecognized directives and directives
// out of place (after the first statement).
func ParseDirectives(f *bzl.File) []Directive {
	return parseDirectives(f.Stmt)
}

// ParseDirectivesFromMacro scans a macro body for Gazelle directives. The
// full list of directives is returned. Errors are reported for unrecognized
// directives and directives out of place (after the first statement).
func ParseDirectivesFromMacro(f *bzl.DefStmt) []Directive {
	return parseDirectives(f.Body)
}

func parseDirectives(stmt []bzl.Expr) []Directive {
	var directives []Directive
	parseComment := func(com bzl.Comment) {
		match := directiveRe.FindStringSubmatch(com.Token)
		if match == nil {
			return
		}
		key, value := match[1], match[2]
		directives = append(directives, Directive{key, value})
	}

	for _, s := range stmt {
		coms := s.Comment()
		for _, com := range coms.Before {
			parseComment(com)
		}
		for _, com := range coms.After {
			parseComment(com)
		}
	}
	return directives
}

var directiveRe = regexp.MustCompile(`^#\s*gazelle:(\w+)\s*(.*?)\s*$`)

// ParseDirectivesFromFile reads a file and extracts Gazelle directives from it.
// Each line is matched against the same pattern used for BUILD file comments
// (# gazelle:key value). Blank lines and comment lines that don't match
// the directive pattern are ignored.
func ParseDirectivesFromFile(filePath string) ([]Directive, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading directive file: %w", err)
	}
	defer f.Close()

	var directives []Directive
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		match := directiveRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		directives = append(directives, Directive{Key: match[1], Value: match[2]})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading directive file: %w", err)
	}
	return directives, nil
}
