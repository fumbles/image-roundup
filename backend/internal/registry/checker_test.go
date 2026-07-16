package registry

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestDockerHubAuthConfigReadsDockerIOAlias(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)

	auth := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	config := `{"auths":{"docker.io":{"auth":"` + auth + `"}}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, ok := dockerHubAuthConfig()
	if !ok {
		t.Fatal("expected docker.io auth config to be found")
	}
	if cfg.Username != "user" || cfg.Password != "pass" {
		t.Fatalf("unexpected auth config: %#v", cfg)
	}
}

func TestIsVersionTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{tag: "1.0.1", want: true},
		{tag: "1.0.1-alpine", want: true},
		{tag: "v1.0.1", want: true},
		{tag: "latest", want: false},
		{tag: "develop", want: false},
		{tag: "nightly", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got := IsVersionTag(tt.tag)
			if got != tt.want {
				t.Fatalf("IsVersionTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectLatestSemverTag(t *testing.T) {
	tests := []struct {
		name       string
		tags       []string
		currentTag string
		platform   string
		repository string
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
		{
			name: "keeps python slim tags in the linux slim family",
			tags: []string{
				"3.12-slim",
				"3.13-slim",
				"3.13-alpine",
				"3.15.0b3-slim",
				"3.15.0b3-windowsservercore-ltsc2025",
			},
			currentTag: "3.12-slim",
			platform:   "linux/amd64",
			want:       "3.13-slim",
		},
		{
			name:       "generic linux tag ignores variant families",
			tags:       []string{"15", "17-alpine", "16"},
			currentTag: "15",
			platform:   "linux/amd64",
			want:       "16",
		},
		{
			name: "postgres alpine stays on current major",
			tags: []string{
				"15-alpine",
				"15.18-alpine3.24",
				"16.10-alpine3.24",
				"18.4-alpine3.24",
			},
			currentTag: "15-alpine",
			platform:   "linux/amd64",
			repository: "index.docker.io/library/postgres",
			want:       "15.18-alpine3.24",
		},
		{
			name: "postgres generic stays on current major",
			tags: []string{
				"17",
				"17.10",
				"18.4",
				"18.4-alpine3.24",
			},
			currentTag: "17",
			platform:   "linux/amd64",
			repository: "index.docker.io/library/postgres",
			want:       "17.10",
		},
		{
			name: "non database images can suggest newer major",
			tags: []string{
				"1.31.1",
				"1.31.2-perl",
				"1.32.0",
			},
			currentTag: "1.31.1",
			platform:   "linux/amd64",
			repository: "index.docker.io/library/nginx",
			want:       "1.32.0",
		},
		{
			name: "nginx generic stream ignores perl variant",
			tags: []string{
				"1.31.2",
				"1.31.3-perl",
				"1.31.3-otel",
				"1.31.3",
			},
			currentTag: "1.31.2",
			platform:   "linux/amd64",
			repository: "index.docker.io/library/nginx",
			want:       "1.31.3",
		},
		{
			name: "nginx generic stream does not cross to otel variant",
			tags: []string{
				"1.31.2",
				"1.31.3-otel",
			},
			currentTag: "1.31.2",
			platform:   "linux/amd64",
			repository: "index.docker.io/library/nginx",
			want:       "",
		},
		{
			name: "nginx perl stream keeps perl variant",
			tags: []string{
				"1.31.2-perl",
				"1.31.3",
				"1.31.3-perl",
			},
			currentTag: "1.31.2-perl",
			platform:   "linux/amd64",
			repository: "index.docker.io/library/nginx",
			want:       "1.31.3-perl",
		},
		{
			name: "v-prefixed dotted stream ignores plain numeric build tags",
			tags: []string{
				"v3.41.1",
				"1243",
				"v3.41.2",
			},
			currentTag: "v3.41.1",
			platform:   "linux/amd64",
			repository: "index.docker.io/qmcgaw/gluetun",
			want:       "v3.41.2",
		},
		{
			name: "v-prefixed dotted stream does not suggest plain numeric tag",
			tags: []string{
				"v3.41.1",
				"1243",
			},
			currentTag: "v3.41.1",
			platform:   "linux/amd64",
			repository: "index.docker.io/qmcgaw/gluetun",
			want:       "",
		},
		{
			name: "app version tag can suggest newer patch",
			tags: []string{
				"latest",
				"1.0.1",
				"1.0.2",
			},
			currentTag: "1.0.1",
			platform:   "linux/amd64",
			repository: "index.docker.io/fumbles/image-roundup",
			want:       "1.0.2",
		},
		{
			name: "linuxserver latest stream ignores develop and nightly tags",
			tags: []string{
				"latest",
				"develop",
				"nightly",
				"2.5.1-develop",
				"2.5.1-nightly",
			},
			currentTag: "latest",
			platform:   "linux/amd64",
			repository: "lscr.io/linuxserver/prowlarr",
			want:       "",
		},
		{
			name: "linuxserver develop stream can suggest newer develop tag",
			tags: []string{
				"latest",
				"develop",
				"2.5.0-develop",
				"2.5.1-develop",
				"2.5.2-nightly",
			},
			currentTag: "develop",
			platform:   "linux/amd64",
			repository: "lscr.io/linuxserver/prowlarr",
			want:       "2.5.1-develop",
		},
		{
			name: "stable alpine stream ignores numeric alpine slim release tags",
			tags: []string{
				"stable-alpine",
				"stable-alpine3.22",
				"1.31.3",
				"1.31.3-alpine3.24-slim",
			},
			currentTag: "stable-alpine",
			platform:   "linux/amd64",
			repository: "index.docker.io/nginxinc/nginx-unprivileged",
			want:       "",
		},
		{
			name: "stable alpine stream ignores other stable variants",
			tags: []string{
				"stable-alpine",
				"stable-perl",
				"stable-otel",
				"1.31.3-alpine3.24",
			},
			currentTag: "stable-alpine",
			platform:   "linux/amd64",
			repository: "index.docker.io/nginxinc/nginx-unprivileged",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectLatestSemverTag(tt.tags, tt.currentTag, tt.platform, tt.repository)
			if got != tt.want {
				t.Fatalf("selectLatestSemverTag() = %q, want %q", got, tt.want)
			}
		})
	}
}
