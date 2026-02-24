package foo
import "embed"
//go:embed q1.txt q2.txt "q 3.txt" `q 4.txt`
var fs embed.FS