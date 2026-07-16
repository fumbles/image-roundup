// Package api provides the HTTP REST API handlers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/yamlwrangler/image-roundup/backend/internal/cache"
	"github.com/yamlwrangler/image-roundup/backend/internal/k8s"
	"github.com/yamlwrangler/image-roundup/backend/internal/models"
)

// Handler bundles the dependencies needed by API handlers.
type Handler struct {
	store     *cache.Store
	scanner   *k8s.Scanner
	log       *zap.Logger
	settings  *models.Settings
	scanOpts  k8s.DiscoveryOptions
	afterScan func() // optional callback invoked after manual scan completes
}

// StaticDir is the directory from which the React SPA is served.
// Defaults to "./static". Set to empty to disable static serving.
var StaticDir = envStaticDir()

func envStaticDir() string {
	if v := os.Getenv("STATIC_DIR"); v != "" {
		return v
	}
	return "./static"
}

// NewHandler creates a Handler.
func NewHandler(
	store *cache.Store,
	scanner *k8s.Scanner,
	log *zap.Logger,
	settings *models.Settings,
	scanOpts k8s.DiscoveryOptions,
	afterScan func(),
) *Handler {
	return &Handler{
		store:     store,
		scanner:   scanner,
		log:       log,
		settings:  settings,
		scanOpts:  scanOpts,
		afterScan: afterScan,
	}
}

// Router builds and returns the chi router.
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(zapLogger(h.log))
	r.Use(middleware.Recoverer)

	// Health probes
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/metrics", promhttp.Handler())

	// Serve static React SPA — must be registered after /api routes
	if StaticDir != "" {
		fs := http.FileServer(http.Dir(StaticDir))
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			// For paths that don't match a real file, serve index.html (SPA fallback)
			path := StaticDir + req.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, req, StaticDir+"/index.html")
				return
			}
			fs.ServeHTTP(w, req)
		})
	}

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/docs", h.getDocs)
		r.Get("/openapi.json", h.getOpenAPI)
		r.Get("/summary", h.getSummary)
		r.Get("/summary/updates", h.getUpdatesSummary)
		r.Get("/images", h.getImages)
		r.Get("/images/{id}", h.getImage)
		r.Get("/registries", h.getRegistries)
		r.Get("/scan", h.getScan)
		r.Post("/scan", h.postScan)
		r.Get("/settings", h.getSettings)
		r.Put("/settings", h.putSettings)
	})

	return r
}

// --- handlers ---

func (h *Handler) getSummary(w http.ResponseWriter, r *http.Request) {
	records := h.store.ListRecords()
	registries := make(map[string]struct{})
	s := models.Summary{
		TotalImages: len(records),
		LastScan:    h.store.LastScan(),
	}
	for _, rec := range records {
		registries[rec.Registry] = struct{}{}
		switch rec.Status {
		case models.StatusUpToDate:
			s.UpToDate++
		case models.StatusUpdateAvailable:
			s.UpdatesAvailable++
		case models.StatusUnknown:
			s.Unknown++
		case models.StatusCheckFailed:
			s.CheckFailed++
		}
	}
	s.UniqueRegistries = len(registries)
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) getUpdatesSummary(w http.ResponseWriter, r *http.Request) {
	records := h.store.ListRecords()
	result := models.UpdatesSummary{
		LastScan: h.store.LastScan(),
		Updates:  []models.UpdateSummary{},
	}

	for _, rec := range records {
		if rec.Status != models.StatusUpdateAvailable {
			continue
		}
		result.Updates = append(result.Updates, updateSummaryFromRecord(rec))
	}

	result.Count = len(result.Updates)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getImages(w http.ResponseWriter, r *http.Request) {
	records := h.store.ListRecords()
	q := r.URL.Query()

	search := strings.ToLower(q.Get("search"))
	ns := q.Get("namespace")
	reg := q.Get("registry")
	kind := q.Get("kind")
	status := q.Get("status")

	var filtered []*models.ImageRecord
	for _, rec := range records {
		if ns != "" && rec.Namespace != ns {
			continue
		}
		if reg != "" && rec.Registry != reg {
			continue
		}
		if kind != "" && !strings.EqualFold(rec.WorkloadKind, kind) {
			continue
		}
		if status != "" && string(rec.Status) != status {
			continue
		}
		if search != "" {
			hay := strings.ToLower(rec.ConfiguredImage + " " + rec.Namespace + " " + rec.WorkloadName + " " + rec.ContainerName)
			if !strings.Contains(hay, search) {
				continue
			}
		}
		filtered = append(filtered, rec)
	}
	if filtered == nil {
		filtered = []*models.ImageRecord{}
	}
	writeJSON(w, http.StatusOK, filtered)
}

func updateSummaryFromRecord(rec *models.ImageRecord) models.UpdateSummary {
	reason := "digest_changed"
	if rec.LatestTag != "" {
		reason = "newer_version_tag"
	}

	latestVersion := rec.LatestTag
	if latestVersion == "" && rec.RegistryDigest != "" && rec.RunningDigest != rec.RegistryDigest {
		latestVersion = "digest changed"
	}

	return models.UpdateSummary{
		ID:              rec.ID,
		Image:           rec.ConfiguredImage,
		CurrentVersion:  rec.Tag,
		LatestVersion:   latestVersion,
		Namespace:       rec.Namespace,
		Workload:        rec.WorkloadKind + "/" + rec.WorkloadName,
		WorkloadKind:    rec.WorkloadKind,
		WorkloadName:    rec.WorkloadName,
		ContainerName:   rec.ContainerName,
		Management:      rec.Management,
		Registry:        rec.Registry,
		Repository:      rec.Repository,
		UpdateReason:    reason,
		LastChecked:     rec.LastChecked,
		RunningDigest:   rec.RunningDigest,
		RegistryDigest:  rec.RegistryDigest,
		LatestTagDigest: rec.LatestTagDigest,
	}
}

func (h *Handler) getImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec := h.store.GetRecord(id)
	if rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (h *Handler) getRegistries(w http.ResponseWriter, r *http.Request) {
	records := h.store.ListRecords()
	type regData struct {
		count     int
		lastError string
	}
	byReg := make(map[string]*regData)
	for _, rec := range records {
		d, ok := byReg[rec.Registry]
		if !ok {
			d = &regData{}
			byReg[rec.Registry] = d
		}
		d.count++
		if rec.Status == models.StatusCheckFailed && rec.Error != "" {
			d.lastError = rec.Error
		}
	}
	var result []models.RegistryInfo
	for host, d := range byReg {
		result = append(result, models.RegistryInfo{
			Hostname:   host,
			ImageCount: d.count,
			LastError:  d.lastError,
		})
	}
	if result == nil {
		result = []models.RegistryInfo{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getScan(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.ScanStatus())
}

func (h *Handler) postScan(w http.ResponseWriter, r *http.Request) {
	status := h.store.ScanStatus()
	if status.Running {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "scan already in progress"})
		return
	}

	req, err := decodeScanRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	scoped := req.Namespace != "" || req.WorkloadKind != "" || req.WorkloadName != ""
	if (req.WorkloadKind != "" || req.WorkloadName != "") && (req.Namespace == "" || req.WorkloadKind == "" || req.WorkloadName == "") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workload scans require namespace, workloadKind, and workloadName"})
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		opts := scopedScanOptions(h.scanOpts, req)
		var err error
		if scoped {
			err = h.scanner.RunScoped(ctx, opts)
		} else {
			err = h.scanner.Run(ctx, opts)
		}
		if err != nil {
			h.log.Error("manual scan failed", zap.Error(err))
			return
		}
		if h.afterScan != nil {
			h.afterScan()
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "scan started"})
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.settings)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	var next models.Settings
	if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	*h.settings = next
	h.scanOpts = scanOptionsFromSettings(next)
	writeJSON(w, http.StatusOK, h.settings)
}

// --- helpers ---

func scanOptionsFromSettings(settings models.Settings) k8s.DiscoveryOptions {
	return k8s.DiscoveryOptions{
		IncludedNamespaces:      settings.IncludedNamespaces,
		ExcludedNamespaces:      settings.ExcludedNamespaces,
		SkipCompleted:           !settings.IncludeCompletedPods,
		ExcludeInternalRegistry: settings.ExcludeInternalRegistry,
	}
}

func decodeScanRequest(r *http.Request) (models.ScanRequest, error) {
	var req models.ScanRequest
	if r.Body == nil {
		return req, nil
	}
	if r.ContentLength == 0 {
		return req, nil
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return req, err
	}
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.WorkloadKind = strings.TrimSpace(req.WorkloadKind)
	req.WorkloadName = strings.TrimSpace(req.WorkloadName)
	return req, nil
}

func scopedScanOptions(base k8s.DiscoveryOptions, req models.ScanRequest) k8s.DiscoveryOptions {
	opts := base
	if req.Namespace != "" {
		opts.IncludedNamespaces = []string{req.Namespace}
	}
	opts.WorkloadKind = req.WorkloadKind
	opts.WorkloadName = req.WorkloadName
	return opts
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// nothing we can do; headers already sent
		_ = err
	}
}

// zapLogger returns a chi middleware that logs requests with zap.
func zapLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t := time.Now()
			next.ServeHTTP(ww, r)
			log.Info("http",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(t)),
			)
		})
	}
}
