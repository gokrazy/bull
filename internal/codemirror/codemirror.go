//go:build !nocodemirror

package codemirror

import _ "embed"

//go:embed bull-codemirror.bundle.js
var BullCodemirror []byte
