// Package registry resolves OCI image digests from remote registries.
package registry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.uber.org/zap"
)

// Checker resolves registry digests for image tags.
type Checker struct {
	log        *zap.Logger
	httpClient *http.Client
}

// NewChecker creates a Checker with the given timeout.
func NewChecker(timeout time.Duration, log *zap.Logger) *Checker {
	return &Checker{
		log: log,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				ResponseHeaderTimeout: timeout,
			},
		},
	}
}

// Result holds the outcome of a registry digest lookup.
type Result struct {
	Digest   string
	Platform string
	Error    error
}

// Resolve fetches the current digest for a tag (e.g. "quay.io/example/app:latest").
// It handles single-arch manifests and multi-arch manifest indexes.
func (c *Checker) Resolve(ctx context.Context, imageRef string) Result {
	ref, err := name.ParseReference(imageRef, name.WeakValidation)
	if err != nil {
		return Result{Error: fmt.Errorf("parsing reference %q: %w", imageRef, err)}
	}

	opts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithTransport(c.httpClient.Transport),
	}

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return Result{Error: fmt.Errorf("fetching manifest for %q: %w", imageRef, err)}
	}

	digest := desc.Digest.String()
	platform := ""

	// If it is a manifest index (multi-arch), pick a sensible platform.
	if strings.Contains(string(desc.MediaType), "manifest.list") ||
		strings.Contains(string(desc.MediaType), "index") {
		platform = "multi-arch"
	}

	return Result{Digest: digest, Platform: platform}
}

// ResolveWithKeychain behaves like Resolve but uses the provided keychain for authentication.
func (c *Checker) ResolveWithKeychain(ctx context.Context, imageRef string, kc authn.Keychain) Result {
	ref, err := name.ParseReference(imageRef, name.WeakValidation)
	if err != nil {
		return Result{Error: fmt.Errorf("parsing reference %q: %w", imageRef, err)}
	}

	opts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(kc),
		remote.WithTransport(c.httpClient.Transport),
	}

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return Result{Error: fmt.Errorf("fetching manifest for %q: %w", imageRef, err)}
	}

	platform := ""
	if strings.Contains(string(desc.MediaType), "manifest.list") ||
		strings.Contains(string(desc.MediaType), "index") {
		platform = "multi-arch"
	}

	return Result{Digest: desc.Digest.String(), Platform: platform}
}
