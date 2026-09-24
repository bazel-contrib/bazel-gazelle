//go:build ignore
// +build ignore

package cgo_ignore_only

/*
void noop(void) {}
*/
import "C"

func noop() {
	C.noop()
}
