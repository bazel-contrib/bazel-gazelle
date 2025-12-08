/* Copyright 2023 The Bazel Authors. All rights reserved.

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

package visibility

import (
	"flag"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const (
	_visibilityDirectiveName = "default_visibility"
	_featureDirectiveName    = "default_features"
)

type visConfig struct {
	visibilityTargets []string
	features          []string
}

// getVisConfig directly returns the internal configuration struct rather
// than a pointer because we explicitly want pass-by-value symantics so
// configurations down a directory tree don't accidentially update upstream.
func getVisConfig(c *config.Config) visConfig {
	cfg := c.Exts[_extName]
	if cfg == nil {
		return visConfig{}
	}
	return cfg.(visConfig)
}

// RegisterFlags noops because we only parameterize behavior with a directive.
func (*visibilityExtension) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}

// CheckFlags noops because no flags are referenced.
func (*visibilityExtension) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}

// KnownDirectives returns the only directive this extension operates on.
func (*visibilityExtension) KnownDirectives() []string {
	return []string{_featureDirectiveName, _visibilityDirectiveName}
}

// Configure identifies the visibility targets from the directive value, if it exists.
//
// To set multiple visibility targets, either multiple directives can be used, or a
// list can be provided with comma-separated values.
func (*visibilityExtension) Configure(c *config.Config, _ string, f *rule.File) {
	cfg := getVisConfig(c)
	if f == nil {
		return
	}

	var newVisTargets []string
	var newFeatures []string
	for _, d := range f.Directives {
		switch d.Key {
		case _visibilityDirectiveName:
			for _, target := range strings.Split(d.Value, ",") {
				newVisTargets = append(newVisTargets, target)
			}
		case _featureDirectiveName:
			for _, feature := range strings.Split(d.Value, ",") {
				newFeatures = append(newFeatures, feature)
			}
		}
	}

	// if visibility targets were specified, overwrite the config
	if len(newVisTargets) != 0 {
		cfg.visibilityTargets = newVisTargets
	}

	if len(newFeatures) != 0 {
		cfg.features = newFeatures
	}

	c.Exts[_extName] = cfg
}

// /Configurator embed
