// Package k8s — scanner orchestrates discovery and registry resolution.
package k8s

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/yamlwrangler/image-roundup/backend/internal/cache"
	"github.com/yamlwrangler/image-roundup/backend/internal/models"
	"github.com/yamlwrangler/image-roundup/backend/internal/registry"
)

// Scanner ties together Kubernetes discovery and registry resolution.
type Scanner struct {
	kc      *Client
	checker *registry.Checker
	store   *cache.Store
	log     *zap.Logger
}

// NewScanner constructs a Scanner.
func NewScanner(kc *Client, checker *registry.Checker, store *cache.Store, log *zap.Logger) *Scanner {
	return &Scanner{kc: kc, checker: checker, store: store, log: log}
}

// Run performs a single full scan and updates the store.
func (s *Scanner) Run(ctx context.Context, opts DiscoveryOptions) error {
	s.store.SetScanning(true)
	defer s.store.SetScanning(false)

	s.log.Info("scan started")

	records, err := s.kc.DiscoverImages(ctx, opts)
	if err != nil {
		return err
	}

	s.log.Info("discovery complete", zap.Int("images", len(records)))

	var scanErrors []string

	for _, rec := range records {
		if rec.Tag == "" {
			rec.Status = models.StatusUnknown
			continue
		}

		registryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		result := s.checker.Resolve(registryCtx, rec.ConfiguredImage)
		cancel()

		now := time.Now().UTC()
		rec.LastChecked = &now

		if result.Error != nil {
			rec.Status = models.StatusCheckFailed
			rec.Error = result.Error.Error()
			scanErrors = append(scanErrors, rec.Error)
			s.log.Warn("registry check failed",
				zap.String("image", rec.ConfiguredImage),
				zap.Error(result.Error))
			continue
		}

		rec.RegistryDigest = result.Digest
		rec.IndexDigest = result.IndexDigest
		if result.Platform != "" {
			rec.Platform = result.Platform
		}

		switch {
		case rec.RunningDigest == "":
			rec.Status = models.StatusUnknown
		case rec.RunningDigest == rec.RegistryDigest:
			rec.Status = models.StatusUpToDate
		default:
			rec.Status = models.StatusUpdateAvailable
		}

		// Only look up the latest semver tag when an update is already flagged.
		// This avoids an extra API call per image on every scan.
		if rec.Status == models.StatusUpdateAvailable {
			latestCtx, latestCancel := context.WithTimeout(ctx, 20*time.Second)
			lt := s.checker.LatestTag(latestCtx, rec.ConfiguredImage, rec.Tag, rec.Platform)
			latestCancel()
			if lt.Error != nil {
				s.log.Debug("latest tag lookup failed",
					zap.String("image", rec.ConfiguredImage),
					zap.Error(lt.Error))
			} else {
				rec.LatestTag = lt.Tag
				rec.LatestTagDigest = lt.Digest
			}
		}
	}

	s.store.SetRecords(records)
	s.store.SetErrors(scanErrors)
	s.log.Info("scan complete", zap.Int("records", len(records)), zap.Int("errors", len(scanErrors)))
	return nil
}

// RunLoop runs scans on the given interval until ctx is done.
// afterScan is called (in the same goroutine) after each successful scan; may be nil.
func (s *Scanner) RunLoop(ctx context.Context, opts DiscoveryOptions, interval time.Duration, afterScan func()) {
	runOnce := func() {
		if err := s.Run(ctx, opts); err != nil {
			s.log.Error("scan failed", zap.Error(err))
			return
		}
		if afterScan != nil {
			afterScan()
		}
	}

	runOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce()
		}
	}
}
