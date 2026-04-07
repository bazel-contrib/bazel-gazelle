package embedsrcs

import "embed"

//go:embed m_static.txt m_go/* deep/nested/deep.txt
var testFS embed.FS
