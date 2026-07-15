package registry

import "testing"

func TestSelectLatestSemverTag(t *testing.T) {
	tests := []struct {
		name       string
		tags       []string
		currentTag string
		platform   string
		want       string
	}{
		{
			name:       "ignores same-version arch suffix for generic current tag",
			tags:       []string{"1.43.3.10828-00f62d37d", "1.43.3.10828-00f62d37d-amd64"},
			currentTag: "1.43.3.10828-00f62d37d",
			platform:   "linux/amd64",
			want:       "",
		},
		{
			name:       "keeps newer generic semver tag",
			tags:       []string{"1.43.3.10828-00f62d37d", "1.44.0.10000-abcd"},
			currentTag: "1.43.3.10828-00f62d37d",
			platform:   "linux/amd64",
			want:       "1.44.0.10000-abcd",
		},
		{
			name:       "keeps same arch suffix for arch-specific current tag",
			tags:       []string{"2.0.0-arm64", "2.0.1-amd64", "2.0.2-arm64"},
			currentTag: "2.0.0-arm64",
			platform:   "linux/arm64",
			want:       "2.0.2-arm64",
		},
		{
			name:       "ignores other arch suffixes for generic current tag",
			tags:       []string{"2.0.0", "2.0.1-arm64"},
			currentTag: "2.0.0",
			platform:   "linux/amd64",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectLatestSemverTag(tt.tags, tt.currentTag, tt.platform)
			if got != tt.want {
				t.Fatalf("selectLatestSemverTag() = %q, want %q", got, tt.want)
			}
		})
	}
}
