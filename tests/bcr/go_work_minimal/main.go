package main

import (
	"fmt"

	"example.com/repro/pkg/foo"
	"example.com/thirdparty"
)

func main() {
	fmt.Println(foo.Name, thirdparty.UsesFoo())
}
