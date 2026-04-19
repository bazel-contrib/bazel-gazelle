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

// Package config provides extensible configuration for Gazelle libraries.
//
// Packages may define Configurers which add support for new command-line
// options and directive comments in build files. Note that the
// language.Language interface embeds Configurer, so each language extension
// has the opportunity
//
// When Gazelle walks the directory trees in a repository, it calls the
// Configure method of each Configurer to produce a Config object.
// Config objects are passed as arguments to most functions in Gazelle, so
// this mechanism may be used to control many aspects of Gazelle's behavior.
package config

import (
	"context"
	"flag"
	"log"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	v2 "github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

// Config holds information about how Gazelle should run. This is based on
// command line arguments, directives, other hints in build files.
//
// A Config applies to a single directory. A Config is created for the
// repository root directory, then copied and modified for each subdirectory.
//
// Config itself contains only general information. Most configuration
// information is language-specific and is stored in Exts. This information
// is modified by extensions that implement Configurer.
//
// Deprecated: use github.com/bazel-contrib/bazel-gazelle/v2/config.Config instead.
//
//go:fix inline
type Config = v2.Config

// MappedKind describes a replacement to use for a built-in kind.
//
// Deprecated: use github.com/bazel-contrib/bazel-gazelle/v2/config.MappedKind instead.
//
//go:fix inline
type MappedKind = v2.MappedKind

// Deprecated: use github.com/bazel-contrib/bazel-gazelle/v2/config.New instead.
//
//go:fix inline
func New() *Config {
	return v2.New()
}

// Deprecated: use github.com/bazel-contrib/bazel-gazelle/v2/config.DefaultValidBuildFileNames instead.
var DefaultValidBuildFileNames = v2.DefaultValidBuildFileNames

// Configurer is the interface for language or library-specific configuration
// extensions. Most (ideally all) modifications to Config should happen
// via this interface.
//
// Deprecated: use github.com/bazel-contrib/bazel-gazelle/v2/config.Configurer instead.
type Configurer interface {
	// RegisterFlags registers command-line flags used by the extension. This
	// method is called once with the root configuration when Gazelle
	// starts. RegisterFlags may set an initial values in Config.Exts. When flags
	// are set, they should modify these values.
	RegisterFlags(fs *flag.FlagSet, cmd string, c *Config)

	// CheckFlags validates the configuration after command line flags are parsed.
	// This is called once with the root configuration when Gazelle starts.
	// CheckFlags may set default values in flags or make implied changes.
	CheckFlags(fs *flag.FlagSet, c *Config) error

	// KnownDirectives returns a list of directive keys that this Configurer can
	// interpret. Gazelle prints errors for directives that are not recoginized by
	// any Configurer.
	KnownDirectives() []string

	// Configure modifies the configuration using directives and other information
	// extracted from a build file. Configure is called in each directory.
	//
	// c is the configuration for the current directory. It starts out as a copy
	// of the configuration for the parent directory.
	//
	// rel is the slash-separated relative path from the repository root to
	// the current directory. It is "" for the root directory itself.
	//
	// f is the build file for the current directory or nil if there is no
	// existing build file.
	Configure(c *Config, rel string, f *rule.File)
}

var _ Configurer = (*CommonConfigurer)(nil)

// CommonConfigurer handles language-agnostic command-line flags and directives,
// i.e., those that apply to Config itself and not to Config.Exts.
//
// Deprecated: use github.com/bazel-contrib/bazel-gazelle/v2/config.CommonConfigurer
// instead.
type CommonConfigurer struct {
	v2 v2.CommonConfigurer
}

func (cc *CommonConfigurer) Name() string {
	return cc.v2.Name()
}

func (cc *CommonConfigurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *Config) {
	cc.v2.RegisterFlags(fs, cmd, c)
}

func (cc *CommonConfigurer) CheckFlags(fs *flag.FlagSet, c *Config) error {
	return cc.v2.CheckFlags(fs, c)
}

func (cc *CommonConfigurer) KnownDirectives() []string {
	return cc.v2.KnownDirectives()
}

func (cc *CommonConfigurer) Configure(c *Config, rel string, f *rule.File) {
	err := cc.v2.Configure(context.Background(), config.ConfigureArgs{
		Config: c,
		Rel:    rel,
		File:   f,
	})
	if err != nil {
		log.Print(err)
	}
}
