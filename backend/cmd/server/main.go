package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/yamlwrangler/image-roundup/backend/internal/api"
	"github.com/yamlwrangler/image-roundup/backend/internal/cache"
	"github.com/yamlwrangler/image-roundup/backend/internal/config"
	"github.com/yamlwrangler/image-roundup/backend/internal/k8s"
	"github.com/yamlwrangler/image-roundup/backend/internal/registry"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	cfg := config.Load()

	kubeClient, err := k8s.New(cfg.InCluster, cfg.KubeConfig, log)
	if err != nil {
		log.Fatal("could not create kubernetes client", zap.Error(err))
	}

	store := cache.New()

	// Load persisted results from the previous run so the UI is not empty on restart.
	dataFile := dataFilePath(cfg.DataDir)
	if dataFile != "" {
		if err := store.Load(dataFile); err != nil {
			log.Warn("could not load persisted data (will rescan)", zap.Error(err))
		} else {
			log.Info("loaded persisted records", zap.String("file", dataFile))
		}
	}

	timeout := time.Duration(cfg.Settings.RegistryTimeoutSeconds) * time.Second
	checker := registry.NewChecker(timeout, log)
	scanner := k8s.NewScanner(kubeClient, checker, store, log)

	scanOpts := k8s.DiscoveryOptions{
		IncludedNamespaces:      cfg.Settings.IncludedNamespaces,
		ExcludedNamespaces:      cfg.Settings.ExcludedNamespaces,
		SkipCompleted:           !cfg.Settings.IncludeCompletedPods,
		ExcludeInternalRegistry: cfg.Settings.ExcludeInternalRegistry,
	}

	// afterScan is called after every completed scan (scheduled or manual).
	var afterScan func()
	if dataFile != "" {
		afterScan = func() {
			if err := store.Save(dataFile); err != nil {
				log.Warn("could not persist scan results", zap.Error(err))
			} else {
				log.Info("scan results persisted", zap.String("file", dataFile))
			}
		}
	}

	settings := cfg.Settings
	handler := api.NewHandler(store, scanner, log, &settings, scanOpts, afterScan)

	interval := time.Duration(cfg.Settings.ScanIntervalSeconds) * time.Second

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupScanOpts := scanOpts
	if cfg.InCluster {
		detectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if current, ok, err := kubeClient.CurrentWorkload(detectCtx); err != nil {
			log.Warn("could not detect current workload for startup scan exclusion", zap.Error(err))
		} else if ok {
			startupScanOpts.ExcludedWorkloads = append(startupScanOpts.ExcludedWorkloads, current)
			log.Info("excluding current workload from startup scan",
				zap.String("namespace", current.Namespace),
				zap.String("kind", current.Kind),
				zap.String("name", current.Name))
		}
		cancel()
	}

	// Start scan loop in background; persist after each completed scan.
	go scanner.RunLoopWithStartupOptions(ctx, startupScanOpts, scanOpts, interval, afterScan)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server listening", zap.String("addr", cfg.ListenAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}

// dataFilePath returns the full path to the records file, or "" if no data dir is configured.
func dataFilePath(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "records.ndjson")
}
