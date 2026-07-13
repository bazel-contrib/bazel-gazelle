package compilerflags

/*
#cgo CFLAGS: -DFROM_SRC
#cgo LDFLAGS: -lfromsrc
*/
import "C"

// Use exists so the package has content.
func Use() {}
