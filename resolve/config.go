/* Copyright 2018 The Bazel Authors. All rights reserved.

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

package resolve

import (
	"context"
	"flag"
	"log"

	configv2 "github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	v2 "github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/bazelbuild/bazel-gazelle/config"
)

// FindRuleWithOverride searches the current configuration for user-specified
// dependency resolution overrides. Overrides specified later (in configuration
// files in deeper directories, or closer to the end of the file) are
// returned first. If no override is found, label.NoLabel is returned.
//
// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/resolve.FindRuleWithOverride instead.
//
//go:fix inline
func FindRuleWithOverride(c *config.Config, imp ImportSpec, lang string) (label.Label, bool) {
	return v2.FindRuleWithOverride(c, imp, lang)
}

// Deprecated: Use github.com/bazel-contrib/bazel-gazelle/v2/resolve.Configurer instead.
type Configurer struct {
	v2 v2.Configurer
}

var _ config.Configurer = (*Configurer)(nil)

func (*Configurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}

func (*Configurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}

func (cc *Configurer) KnownDirectives() []string {
	return cc.v2.KnownDirectives()
}

func (cc *Configurer) Configure(c *config.Config, rel string, f *rule.File) {
	err := cc.v2.Configure(context.Background(), configv2.ConfigureArgs{
		Config: c,
		Rel:    rel,
		File:   f,
	})
	if err != nil {
		log.Print(err)
	}
}
