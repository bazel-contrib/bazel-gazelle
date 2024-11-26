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
	"errors"
	"flag"
	"fmt"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

type conventionConfig struct {
	// genResolves controls whether or not gazelle will add resolve directives
	// for non-conventional imports to the top level BUILD.bazel.
	genResolves bool
	// recursiveMode denotes if Gazelle was set to run recursively via the "-r" flag
	recursiveMode bool
}

const _conventionName = "_convention"

// Configurer is convention's implementation of the config.Configurer interface.
type Configurer struct{}

func getConventionConfig(c *config.Config) *conventionConfig {
	cc := c.Exts[_conventionName]
	if cc == nil {
		return &conventionConfig{}
	}
	return cc.(*conventionConfig)
}

// RegisterFlags registers the genResolves flag, used to enable/disable this library.
func (*Configurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	cc := getConventionConfig(c)
	switch cmd {
	case "fix", "update":
		fs.BoolVar(&cc.genResolves, "resolveGen", false, "whether gazelle will add resolve directives for non-conventional imports in the top level BUILD.bazel")
	}
	c.Exts[_conventionName] = cc
}

// getRecursiveMode looks up the "r" flag's value, which should be registered by
// the upstream cmd/gazelle/fix-update.go Configurer.
func getRecursiveMode(fs *flag.FlagSet) (bool, error) {
	f := fs.Lookup("r")
	if f == nil {
		return false, errors.New("expected -r flag to be set")
	}
	g, ok := f.Value.(flag.Getter)
	if !ok {
		return false, fmt.Errorf("got data of type %T but wanted flag.Getter", g)
	}
	recursiveMode, ok := g.Get().(bool)
	if !ok {
		return false, fmt.Errorf("got data of type %T but wanted bool", recursiveMode)
	}
	return recursiveMode, nil
}

// CheckFlags determines the value of conventionConfig.recursiveMode
func (*Configurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	cc := getConventionConfig(c)
	var err error
	cc.recursiveMode, err = getRecursiveMode(fs)
	return err
}

// KnownDirectives implements config.Configurer interface.
func (*Configurer) KnownDirectives() []string { return nil }

// Configure implements config.Configurer interface.
func (*Configurer) Configure(c *config.Config, rel string, f *rule.File) {
	cc := *getConventionConfig(c)
	c.Exts[_conventionName] = &cc
}
