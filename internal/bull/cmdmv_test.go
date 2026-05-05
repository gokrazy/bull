package bull

import "testing"

func TestReplaceWikilinkTargets(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		oldpg, newpg string
		want         string
	}{
		{
			name:  "bare",
			src:   "see [[Foo]] please",
			oldpg: "Foo", newpg: "Quux",
			want: "see [[Quux]] please",
		},
		{
			name:  "with_label",
			src:   "see [[Foo|some label]] please",
			oldpg: "Foo", newpg: "Quux",
			want: "see [[Quux|some label]] please",
		},
		{
			name:  "with_fragment",
			src:   "see [[Foo#section]] please",
			oldpg: "Foo", newpg: "Quux",
			want: "see [[Quux#section]] please",
		},
		{
			name:  "fragment_and_label",
			src:   "see [[Foo#section|the label]] please",
			oldpg: "Foo", newpg: "Quux",
			want: "see [[Quux#section|the label]] please",
		},
		{
			name:  "table_escape_separator",
			src:   "| [[Foo\\|label]] |",
			oldpg: "Foo", newpg: "Quux",
			want: "| [[Quux\\|label]] |",
		},
		{
			name:  "embed",
			src:   "![[Foo|alt]]",
			oldpg: "Foo", newpg: "Quux",
			want: "![[Quux|alt]]",
		},
		{
			name:  "embed_table_escape",
			src:   "| ![[Foo\\|alt]] |",
			oldpg: "Foo", newpg: "Quux",
			want: "| ![[Quux\\|alt]] |",
		},
		{
			name:  "non_match_left_alone",
			src:   "[[Other]] and [[Foo]] and [[Bar]]",
			oldpg: "Foo", newpg: "Quux",
			want: "[[Other]] and [[Quux]] and [[Bar]]",
		},
		{
			name:  "multiple_occurrences",
			src:   "[[Foo]] then [[Foo|x]] then [[Foo#y|z]]",
			oldpg: "Foo", newpg: "Quux",
			want: "[[Quux]] then [[Quux|x]] then [[Quux#y|z]]",
		},
		{
			name:  "page_path_with_slash",
			src:   "[[Performance/SIMD|fast]]",
			oldpg: "Performance/SIMD", newpg: "perf/simd",
			want: "[[perf/simd|fast]]",
		},
		{
			name:  "trailing_backslash_target_not_matched",
			src:   "[[Foo\\]]",
			oldpg: "Foo", newpg: "Quux",
			want: "[[Foo\\]]", // target is "Foo\", not "Foo"
		},
		{
			name:  "no_close_left_alone",
			src:   "[[Foo and more text",
			oldpg: "Foo", newpg: "Quux",
			want: "[[Foo and more text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(replaceWikilinkTargets([]byte(tt.src), tt.oldpg, tt.newpg))
			if got != tt.want {
				t.Errorf("replaceWikilinkTargets(%q, %q, %q):\n got: %q\nwant: %q",
					tt.src, tt.oldpg, tt.newpg, got, tt.want)
			}
		})
	}
}
