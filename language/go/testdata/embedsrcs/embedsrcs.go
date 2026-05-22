package embedsrcs

import "embed"

//go:embed *m_* n_/* p_dir/* all:o* deep/nested/deep.txt
var fs embed.FS
