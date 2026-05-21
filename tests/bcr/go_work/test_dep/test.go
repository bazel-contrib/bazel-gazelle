package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatch(t *testing.T) {
	// A patch is used to add this constant with a value that differs from that patched into the
	// root module's version of this dep.
	require.Equal(t, "not_hello", require.Hello)
	require.Equal(t, "not_patch_cmd_hello", require.PatchCmdHello)
}
