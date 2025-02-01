package bull

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime/debug"
)

func (b *bullServer) buildinfo(w http.ResponseWriter, r *http.Request) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("debug.ReadBuildInfo() failed")
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[debug.ReadBuildInfo](https://pkg.go.dev/runtime/debug#ReadBuildInfo) returned:\n```\n%+v```\n", info)
	return b.renderBullMarkdown(w, r, "buildinfo", &buf)
}
