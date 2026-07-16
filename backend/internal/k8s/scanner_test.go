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

func TestManagementFromMetadataDetectsHelm(t *testing.T) {
	got := managementFromMetadata(
		map[string]string{"app.kubernetes.io/managed-by": "Helm"},
		map[string]string{
			"meta.helm.sh/release-name":      "kyverno",
			"meta.helm.sh/release-namespace": "kyverno",
		},
	)
	if got == nil {
		t.Fatal("expected Helm management info")
	}
	if got.Tool != "Helm" || got.HelmReleaseName != "kyverno" || got.HelmReleaseNamespace != "kyverno" {
		t.Fatalf("unexpected management info: %#v", got)
	}
}

func TestManagementFromMetadataIgnoresUnmanaged(t *testing.T) {
	if got := managementFromMetadata(nil, nil); got != nil {
		t.Fatalf("managementFromMetadata() = %#v, want nil", got)
	}
}

func TestScanWorkerCount(t *testing.T) {
	tests := []struct {
		records int
		want    int
	}{
		{records: 0, want: 1},
		{records: 3, want: 3},
		{records: 8, want: 8},
		{records: 304, want: 8},
	}

	for _, tt := range tests {
		if got := scanWorkerCount(tt.records); got != tt.want {
			t.Fatalf("scanWorkerCount(%d) = %d, want %d", tt.records, got, tt.want)
		}
	}
}

func TestGroupScanJobsDeduplicatesLookupImages(t *testing.T) {
	records := []*models.ImageRecord{
		{ConfiguredImage: "docker.io/library/postgres:17", Registry: "docker.io"},
		{ConfiguredImage: "docker.io/library/postgres:17", Registry: "docker.io"},
		{ConfiguredImage: "docker.io/library/nginx:1.31", Registry: "docker.io"},
	}

	jobs := groupScanJobs(records, registryLookup{})
	if len(jobs) != 2 {
		t.Fatalf("groupScanJobs() produced %d jobs, want 2", len(jobs))
	}

	for _, job := range jobs {
		if job.lookupImage == "docker.io/library/postgres:17" && len(job.records) != 2 {
			t.Fatalf("postgres job has %d records, want 2", len(job.records))
		}
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
