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
			ID:              "lab:Deployment:dashy:dashy",
			Namespace:       "lab",
			WorkloadKind:    "Deployment",
			WorkloadName:    "dashy",
			ContainerName:   "dashy",
			ConfiguredImage: "lissy93/dashy:latest",
			Registry:        "docker.io",
			Repository:      "lissy93/dashy",
			Tag:             "latest",
			LatestTag:       "4.4.5",
			Status:          models.StatusUpdateAvailable,
		},
	})

	handler := NewHandler(store, nil, nil, zap.NewNop(), &models.Settings{}, k8s.DiscoveryOptions{}, nil)
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
	if update.Image != "lissy93/dashy:latest" {
		t.Fatalf("image = %q", update.Image)
	}
	if update.CurrentVersion != "latest" || update.LatestVersion != "4.4.5" {
		t.Fatalf("versions = %q -> %q", update.CurrentVersion, update.LatestVersion)
	}
	if update.Workload != "Deployment/dashy" {
		t.Fatalf("workload = %q", update.Workload)
	}
	if update.UpdateReason != "newer_version_tag" {
		t.Fatalf("updateReason = %q", update.UpdateReason)
	}
	if update.Management != nil {
		t.Fatalf("management = %#v", update.Management)
	}
}

func TestGetSummarySuppressesHelmManagedImageUpdates(t *testing.T) {
	store := cache.New()
	store.SetRecords([]*models.ImageRecord{
		{
			ID:            "helm:Deployment:policy-reporter:policy-reporter",
			Namespace:     "policy-reporter",
			WorkloadKind:  "Deployment",
			WorkloadName:  "policy-reporter",
			ContainerName: "policy-reporter",
			Management: &models.ManagementInfo{
				Tool:                 "Helm",
				HelmReleaseName:      "policy-reporter",
				HelmReleaseNamespace: "policy-reporter",
			},
			Registry: "ghcr.io",
			Status:   models.StatusUpdateAvailable,
		},
		{
			ID:       "lab:Deployment:dashy:dashy",
			Registry: "docker.io",
			Status:   models.StatusUpdateAvailable,
		},
		{
			ID:       "media:Deployment:plex:plex",
			Registry: "docker.io",
			Status:   models.StatusUpToDate,
		},
	})

	handler := NewHandler(store, nil, nil, zap.NewNop(), &models.Settings{}, k8s.DiscoveryOptions{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/summary", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got models.Summary
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if got.TotalImages != 3 {
		t.Fatalf("totalImages = %d, want 3", got.TotalImages)
	}
	if got.UpdatesAvailable != 1 {
		t.Fatalf("updatesAvailable = %d, want 1", got.UpdatesAvailable)
	}
	if got.UpToDate != 1 {
		t.Fatalf("upToDate = %d, want 1", got.UpToDate)
	}
}

func TestGetImagesUpdateFilterSuppressesHelmManagedImageUpdates(t *testing.T) {
	store := cache.New()
	store.SetRecords([]*models.ImageRecord{
		{
			ID:            "helm:Deployment:policy-reporter:policy-reporter",
			Namespace:     "policy-reporter",
			WorkloadKind:  "Deployment",
			WorkloadName:  "policy-reporter",
			ContainerName: "policy-reporter",
			Management: &models.ManagementInfo{
				Tool:                 "Helm",
				HelmReleaseName:      "policy-reporter",
				HelmReleaseNamespace: "policy-reporter",
			},
			Status: models.StatusUpdateAvailable,
		},
		{
			ID:            "lab:Deployment:dashy:dashy",
			Namespace:     "lab",
			WorkloadKind:  "Deployment",
			WorkloadName:  "dashy",
			ContainerName: "dashy",
			Status:        models.StatusUpdateAvailable,
		},
	})

	handler := NewHandler(store, nil, nil, zap.NewNop(), &models.Settings{}, k8s.DiscoveryOptions{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/images?status=update_available", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []models.ImageRecord
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "lab:Deployment:dashy:dashy" {
		t.Fatalf("id = %q", got[0].ID)
	}
}
