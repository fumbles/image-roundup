package chartrepo

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReadIndexBodyRejectsOversizedContentLength(t *testing.T) {
	_, err := readIndexBody(strings.NewReader("apiVersion: v1\nentries: {}\n"), maxIndexBytes+1, "https://example.test/index.yaml")
	if err == nil {
		t.Fatal("expected oversized index error")
	}
	if !strings.Contains(err.Error(), "limit is") {
		t.Fatalf("error = %q, want size limit error", err.Error())
	}
}

func TestReadIndexBodyRejectsOversizedStreamingBody(t *testing.T) {
	_, err := readIndexBody(strings.NewReader(strings.Repeat("a", maxIndexBytes+1)), -1, "https://example.test/index.yaml")
	if err == nil {
		t.Fatal("expected oversized index error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %q, want size limit error", err.Error())
	}
}

func TestSmallIndexParses(t *testing.T) {
	data, err := readIndexBody(strings.NewReader(`
apiVersion: v1
entries:
  kyverno:
    - version: 3.8.2
      appVersion: v1.18.2
`), -1, "https://example.test/index.yaml")
	if err != nil {
		t.Fatalf("readIndexBody() error = %v", err)
	}

	var index repoIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	entries := index.Entries["kyverno"]
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Version != "3.8.2" || entries[0].AppVersion != "v1.18.2" {
		t.Fatalf("entry = %#v", entries[0])
	}
}
