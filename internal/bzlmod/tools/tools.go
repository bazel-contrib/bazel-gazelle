//go:build tools

package tools

import (
	// Needed by isolated usages of the go_deps extension.
	_ "github.com/golang/protobuf/proto"
	_ "google.golang.org/protobuf"
)
