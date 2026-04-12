//go:build !nomermaid

package mermaid

import _ "embed"

//go:embed bull-mermaid.bundle.js
var BullMermaid []byte
