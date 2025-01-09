// Copyright 2024 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package convention

import (
	"flag"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
)

func TestConfig(t *testing.T) {
	c := config.New()
	cc := &conventionConfig{}
	c.Exts[_conventionName] = cc
	fs := &flag.FlagSet{}
	fs.Bool("r", false, "")
	cext := Configurer{}
	cext.RegisterFlags(fs, "update", c)
	err := fs.Parse([]string{"-r=true", "-resolveGen=true"})
	if err != nil {
		t.Errorf("err should be nil: %v", err)
	}
	if err := cext.CheckFlags(fs, c); err != nil {
		t.Errorf("cext.CheckFlags err should be nil: %v", err)
	}
	if !cc.genResolves {
		t.Error("cc.genResolves should be true")
	}
	if !cc.recursiveMode {
		t.Error("cc.recursiveMode should be true")
	}
}
