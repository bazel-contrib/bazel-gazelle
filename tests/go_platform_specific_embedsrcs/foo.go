
// +build windows

package foo

import _ "embed"

//go:embed windows.txt
var s string
