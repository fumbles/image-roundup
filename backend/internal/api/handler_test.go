package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/yamlwrangler/image-roundup/backend/internal/cache"
	"github.com/yamlwrangler/image-roundup/backend/internal/k8s"
	"github.com/yamlwrangler/image-roundup/backend/internal/models"
)

func TestGetUpdatesSummary(t *testing.T) {
	store := cache.New()
	store.SetRecords([]*models.ImageRecord{
		{
			ID:            "image-roundup:Deployment:image-roundup:image-roundup",
			Namespace:     "image-roundup",
			WorkloadKind:  "Deployment",
			WorkloadName:  "image-roundup",
			ContainerName: "image-roundup",
			Management: &models.ManagementInfo{
				Tool:                 "Helm",
				ManagedBy:            "Helm",
				HelmReleaseName:      "image-roundup",
				HelmReleaseNamespace: "image-roundup",
			},
			ConfiguredImage: "docker.io/fumbles/image-roundup:1.0.1",
			Registry:        "docker.io",
			Repository:      "fumbles/image-roundup",
			Tag:             "1.0.1",
			LatestTag:       "1.0.2",
			Status:          models.StatusUpdateAvailable,
		},
		{
			ID:              "media:Deployment:plex:plex",
			Namespace:       "media",
			WorkloadKind:    "Deployment",
			WorkloadName:    "plex",
			ContainerName:   "plex",
			ConfiguredImage: "plexinc/pms-docker:latest",
			Status:          models.StatusUpToDate,
		},
	})

	handler := NewHandler(store, nil, zap.NewNop(), &models.Settings{}, k8s.DiscoveryOptions{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/summary/updates", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got models.UpdatesSummary
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if got.Count != 1 || len(got.Updates) != 1 {
		t.Fatalf("got count=%d len=%d, want 1 update", got.Count, len(got.Updates))
	}
	update := got.Updates[0]
	if update.Image != "docker.io/fumbles/image-roundup:1.0.1" {
		t.Fatalf("image = %q", update.Image)
	}
	if update.CurrentVersion != "1.0.1" || update.LatestVersion != "1.0.2" {
		t.Fatalf("versions = %q -> %q", update.CurrentVersion, update.LatestVersion)
	}
	if update.Workload != "Deployment/image-roundup" {
		t.Fatalf("workload = %q", update.Workload)
	}
	if update.UpdateReason != "newer_version_tag" {
		t.Fatalf("updateReason = %q", update.UpdateReason)
	}
	if update.Management == nil || update.Management.Tool != "Helm" || update.Management.HelmReleaseName != "image-roundup" {
		t.Fatalf("management = %#v", update.Management)
	}
}
