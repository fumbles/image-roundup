package k8s

import (
	"testing"

	"github.com/yamlwrangler/image-roundup/backend/internal/models"
)

func TestDigestMatches(t *testing.T) {
	tests := []struct {
		name     string
		running  string
		registry string
		index    string
		want     bool
	}{
		{
			name:     "matches platform digest",
			running:  "sha256:platform",
			registry: "sha256:platform",
			index:    "sha256:index",
			want:     true,
		},
		{
			name:     "matches index digest",
			running:  "sha256:index",
			registry: "sha256:platform",
			index:    "sha256:index",
			want:     true,
		},
		{
			name:     "does not match either digest",
			running:  "sha256:old",
			registry: "sha256:platform",
			index:    "sha256:index",
			want:     false,
		},
		{
			name:     "missing running digest does not match",
			registry: "sha256:platform",
			index:    "sha256:index",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := digestMatches(tt.running, tt.registry, tt.index)
			if got != tt.want {
				t.Fatalf("digestMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryLookupImageRef(t *testing.T) {
	lookup := registryLookup{openShiftRouteHost: "default-route-openshift-image-registry.apps.example.com"}

	got := lookup.imageRef(
		"image-registry.openshift-image-registry.svc:5000/app/image:tag",
		"image-registry.openshift-image-registry.svc:5000",
	)
	want := "default-route-openshift-image-registry.apps.example.com/app/image:tag"
	if got != want {
		t.Fatalf("imageRef() = %q, want %q", got, want)
	}

	got = lookup.imageRef("quay.io/example/app:tag", "quay.io")
	if got != "quay.io/example/app:tag" {
		t.Fatalf("non-OpenShift registry was rewritten to %q", got)
	}
}

func TestIsOpenShiftInternalRegistry(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{host: "image-registry.openshift-image-registry.svc:5000", want: true},
		{host: "image-registry.openshift-image-registry.svc.cluster.local:5000", want: true},
		{host: "default-route-openshift-image-registry.apps.example.com", want: false},
		{host: "quay.io", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := isOpenShiftInternalRegistry(tt.host)
			if got != tt.want {
				t.Fatalf("isOpenShiftInternalRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNamespaceExcludedSupportsPrefixWildcard(t *testing.T) {
	opts := DiscoveryOptions{ExcludedNamespaces: []string{"kube-system", "openshift*"}}

	tests := []struct {
		namespace string
		want      bool
	}{
		{namespace: "kube-system", want: true},
		{namespace: "openshift-gitops", want: true},
		{namespace: "openshift", want: true},
		{namespace: "image-roundup", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := isNamespaceExcluded(tt.namespace, opts)
			if got != tt.want {
				t.Fatalf("isNamespaceExcluded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsWorkloadExcluded(t *testing.T) {
	opts := DiscoveryOptions{ExcludedWorkloads: []WorkloadRef{
		{Namespace: "image-roundup", Kind: "Deployment", Name: "image-roundup"},
	}}

	if !isWorkloadExcluded("image-roundup", ownerRef{Kind: "Deployment", Name: "image-roundup"}, opts) {
		t.Fatal("expected image-roundup deployment to be excluded")
	}
	if isWorkloadExcluded("image-roundup", ownerRef{Kind: "Deployment", Name: "other"}, opts) {
		t.Fatal("expected other deployment to be included")
	}
	if isWorkloadExcluded("lab", ownerRef{Kind: "Deployment", Name: "image-roundup"}, opts) {
		t.Fatal("expected matching name in another namespace to be included")
	}
}

func TestScopedRecordMatcher(t *testing.T) {
	matches := scopedRecordMatcher(DiscoveryOptions{
		IncludedNamespaces: []string{"media"},
		WorkloadKind:       "Deployment",
		WorkloadName:       "plex",
	})

	if !matches(&models.ImageRecord{Namespace: "media", WorkloadKind: "Deployment", WorkloadName: "plex"}) {
		t.Fatal("expected scoped matcher to match target workload")
	}
	if matches(&models.ImageRecord{Namespace: "media", WorkloadKind: "Deployment", WorkloadName: "radarr"}) {
		t.Fatal("expected scoped matcher to ignore other workloads")
	}
	if matches(&models.ImageRecord{Namespace: "lab", WorkloadKind: "Deployment", WorkloadName: "plex"}) {
		t.Fatal("expected scoped matcher to ignore other namespaces")
	}
}
