// Package k8s — scanner orchestrates discovery and registry resolution.
package k8s

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
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

	scanErrors := s.checkRecords(ctx, records)
	s.store.SetRecords(records)
	s.store.SetErrors(scanErrors)
	s.log.Info("scan complete", zap.Int("records", len(records)), zap.Int("errors", len(scanErrors)))
	return nil
}

// RunScoped performs a scan limited by opts and replaces only cached records
// matching the same scope.
func (s *Scanner) RunScoped(ctx context.Context, opts DiscoveryOptions) error {
	s.store.SetScanning(true)
	defer s.store.SetScanning(false)

	s.log.Info("scoped scan started",
		zap.Strings("namespaces", opts.IncludedNamespaces),
		zap.String("workloadKind", opts.WorkloadKind),
		zap.String("workloadName", opts.WorkloadName))

	records, err := s.kc.DiscoverImages(ctx, opts)
	if err != nil {
		return err
	}

	s.log.Info("scoped discovery complete", zap.Int("images", len(records)))

	scanErrors := s.checkRecords(ctx, records)
	s.store.ReplaceWhere(records, scopedRecordMatcher(opts))
	s.store.SetErrors(scanErrors)
	s.log.Info("scoped scan complete", zap.Int("records", len(records)), zap.Int("errors", len(scanErrors)))
	return nil
}

func (s *Scanner) checkRecords(ctx context.Context, records []*models.ImageRecord) []string {
	var scanErrors []string
	lookup := s.registryLookup(ctx, records)

	for _, rec := range records {
		lookupImage := lookup.imageRef(rec.ConfiguredImage, rec.Registry)
		lookupAuth := lookup.authenticator(rec.Registry)
		registryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		result := s.checker.ResolveWithAuth(registryCtx, lookupImage, lookupAuth)
		cancel()

		now := time.Now().UTC()
		rec.LastChecked = &now

		if result.Error != nil {
			rec.Status = models.StatusCheckFailed
			rec.Error = result.Error.Error()
			scanErrors = append(scanErrors, rec.Error)
			s.log.Warn("registry check failed",
				zap.String("image", rec.ConfiguredImage),
				zap.String("lookupImage", lookupImage),
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
		case digestMatches(rec.RunningDigest, rec.RegistryDigest, rec.IndexDigest):
			rec.Status = models.StatusUpToDate
		default:
			rec.Status = models.StatusUpdateAvailable
		}

		// Only look up the latest semver tag when an update is already flagged.
		// This avoids an extra API call per image on every scan.
		if rec.Status == models.StatusUpdateAvailable && rec.Tag != "" {
			latestCtx, latestCancel := context.WithTimeout(ctx, 20*time.Second)
			lt := s.checker.LatestTagWithAuth(latestCtx, lookupImage, rec.Tag, rec.Platform, lookupAuth)
			latestCancel()
			if lt.Error != nil {
				s.log.Debug("latest tag lookup failed",
					zap.String("image", rec.ConfiguredImage),
					zap.String("lookupImage", lookupImage),
					zap.Error(lt.Error))
			} else {
				rec.LatestTag = lt.Tag
				rec.LatestTagDigest = lt.Digest
			}
		}
	}

	return scanErrors
}

func scopedRecordMatcher(opts DiscoveryOptions) func(*models.ImageRecord) bool {
	namespaces := make(map[string]struct{}, len(opts.IncludedNamespaces))
	for _, ns := range opts.IncludedNamespaces {
		namespaces[ns] = struct{}{}
	}
	return func(rec *models.ImageRecord) bool {
		if len(namespaces) > 0 {
			if _, ok := namespaces[rec.Namespace]; !ok {
				return false
			}
		}
		if opts.WorkloadKind != "" && !strings.EqualFold(rec.WorkloadKind, opts.WorkloadKind) {
			return false
		}
		if opts.WorkloadName != "" && rec.WorkloadName != opts.WorkloadName {
			return false
		}
		return true
	}
}

func digestMatches(running, registry, index string) bool {
	return running != "" && (running == registry || (index != "" && running == index))
}

type registryLookup struct {
	openShiftRouteHost string
	openShiftAuth      authn.Authenticator
}

func (s *Scanner) registryLookup(ctx context.Context, records []*models.ImageRecord) registryLookup {
	needsOpenShiftRoute := false
	for _, rec := range records {
		if isOpenShiftInternalRegistry(rec.Registry) {
			needsOpenShiftRoute = true
			break
		}
	}
	if !needsOpenShiftRoute {
		return registryLookup{}
	}

	host, err := s.kc.OpenShiftImageRegistryRouteHost(ctx)
	if err != nil {
		s.log.Warn("could not detect OpenShift image registry route", zap.Error(err))
		return registryLookup{}
	}

	auth, err := openShiftServiceAccountAuth()
	if err != nil {
		s.log.Warn("could not load OpenShift service account token for registry route", zap.Error(err))
	} else {
		s.log.Info("using OpenShift service account token for registry route auth")
	}

	s.log.Info("using OpenShift image registry route for lookups", zap.String("host", host), zap.Bool("authConfigured", auth != nil))
	return registryLookup{openShiftRouteHost: host, openShiftAuth: auth}
}

func (l registryLookup) imageRef(configuredImage, registryHost string) string {
	if l.openShiftRouteHost == "" || !isOpenShiftInternalRegistry(registryHost) {
		return configuredImage
	}

	prefix := registryHost + "/"
	if !strings.HasPrefix(configuredImage, prefix) {
		return configuredImage
	}
	return l.openShiftRouteHost + strings.TrimPrefix(configuredImage, registryHost)
}

func (l registryLookup) authenticator(registryHost string) authn.Authenticator {
	if l.openShiftRouteHost == "" || l.openShiftAuth == nil || !isOpenShiftInternalRegistry(registryHost) {
		return nil
	}
	return l.openShiftAuth
}

func openShiftServiceAccountAuth() (authn.Authenticator, error) {
	const tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return nil, os.ErrInvalid
	}
	return &authn.Bearer{Token: token}, nil
}

func isOpenShiftInternalRegistry(registryHost string) bool {
	host := stripRegistryPort(registryHost)
	return host == "image-registry.openshift-image-registry.svc" ||
		host == "image-registry.openshift-image-registry.svc.cluster.local"
}

func stripRegistryPort(registryHost string) string {
	host, _, err := net.SplitHostPort(registryHost)
	if err == nil {
		return host
	}
	if strings.Count(registryHost, ":") == 1 {
		return strings.SplitN(registryHost, ":", 2)[0]
	}
	return registryHost
}

// RunLoop runs scans on the given interval until ctx is done.
// afterScan is called (in the same goroutine) after each successful scan; may be nil.
func (s *Scanner) RunLoop(ctx context.Context, opts DiscoveryOptions, interval time.Duration, afterScan func()) {
	s.RunLoopWithStartupOptions(ctx, opts, opts, interval, afterScan)
}

// RunLoopWithStartupOptions runs an immediate startup scan with startupOpts,
// then recurring scans with opts.
func (s *Scanner) RunLoopWithStartupOptions(ctx context.Context, startupOpts, opts DiscoveryOptions, interval time.Duration, afterScan func()) {
	runOnce := func() {
		if err := s.Run(ctx, opts); err != nil {
			s.log.Error("scan failed", zap.Error(err))
			return
		}
		if afterScan != nil {
			afterScan()
		}
	}

	runStartup := func() {
		if err := s.Run(ctx, startupOpts); err != nil {
			s.log.Error("startup scan failed", zap.Error(err))
			return
		}
		if afterScan != nil {
			afterScan()
		}
	}

	runStartup()
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
