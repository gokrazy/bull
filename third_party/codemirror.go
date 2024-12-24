//go:build !nocodemirror

package thirdparty

import _ "embed"

//go:embed codemirror/bull-codemirror.bundle.js
var BullCodemirror []byte
