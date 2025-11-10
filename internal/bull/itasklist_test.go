package bull

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestToggleCheckbox(t *testing.T) {
	content := `- [ ] foo
- [ ] bar
- [ ] baz
`
	got := toggleCheckbox(content, 2)
	want := `- [ ] foo
- [x] bar
- [ ] baz
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("toggleCheckbox: unexpected diff (-want +got):\n%s", diff)
	}
}
