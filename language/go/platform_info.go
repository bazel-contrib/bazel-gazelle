//go:build gazelle_bootstrap
// +build gazelle_bootstrap

package golang

import (
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// When this library is built via bazel, these implementations
// are replaced with optimized switch statements.

func IsKnownOS(os string) bool {
	return rule.KnownOSSet[os]
}

func IsKnownArch(arch string) bool {
	return rule.KnownArchSet[arch]
}
