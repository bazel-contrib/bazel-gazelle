package thirdparty

import "example.com/repro/pkg/foo"

func UsesFoo() string {
	return foo.Name
}
