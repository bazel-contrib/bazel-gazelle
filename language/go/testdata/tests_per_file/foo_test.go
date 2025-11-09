package tests_per_file

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/testtools"
)

type fileSpec testtools.FileSpec

func TestStuff(t *testing.T) { t.Parallel()}

func TestFoo(t *testing.T) { t.Parallel()}
