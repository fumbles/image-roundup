package k8s

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDiscoverHelmReleasesUsesLatestRevision(t *testing.T) {
	client := &Client{
		kc:  fake.NewSimpleClientset(helmReleaseSecret(t, "kyverno", "kyverno", 1, "3.8.1"), helmReleaseSecret(t, "kyverno", "kyverno", 2, "3.8.2")),
		log: zap.NewNop(),
	}

	releases, err := client.DiscoverHelmReleases(t.Context(), DiscoveryOptions{})
	if err != nil {
		t.Fatalf("DiscoverHelmReleases() error = %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("DiscoverHelmReleases() returned %d releases, want 1", len(releases))
	}
	got := releases[0]
	if got.Name != "kyverno" || got.Namespace != "kyverno" || got.Revision != 2 || got.ChartVersion != "3.8.2" {
		t.Fatalf("release = %#v, want kyverno revision 2 chart 3.8.2", got)
	}
}

func helmReleaseSecret(t *testing.T, namespace, name string, revision int, chartVersion string) *corev1.Secret {
	t.Helper()

	payload := map[string]any{
		"name":      name,
		"namespace": namespace,
		"version":   revision,
		"info": map[string]any{
			"status": "deployed",
		},
		"chart": map[string]any{
			"metadata": map[string]any{
				"name":       name,
				"version":    chartVersion,
				"appVersion": "v1.18.2",
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	var gzipped bytes.Buffer
	writer := gzip.NewWriter(&gzipped)
	if _, err := writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1." + name + ".v" + strconv.Itoa(revision),
			Namespace: namespace,
			Labels: map[string]string{
				"name":    name,
				"owner":   "helm",
				"status":  "deployed",
				"version": strconv.Itoa(revision),
			},
		},
		Type: corev1.SecretType("helm.sh/release.v1"),
		Data: map[string][]byte{
			"release": []byte(base64.StdEncoding.EncodeToString(gzipped.Bytes())),
		},
	}
}
