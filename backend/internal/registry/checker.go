// Package registry resolves OCI image digests from remote registries.
package registry

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
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
	// Digest is what should be compared against the running container digest.
	// For multi-arch images this is the platform-specific (linux/amd64) digest,
	// not the manifest list index digest.
	Digest string

	// IndexDigest is the manifest list digest for multi-arch images (the
	// "sha256:a6d3…" style shown on Docker Hub). Empty for single-arch images.
	IndexDigest string

	Platform string
	Error    error
}

// Resolve fetches the current digest for a tag (e.g. "quay.io/example/app:latest").
//
// For single-arch images it returns the manifest digest directly.
// For multi-arch manifest indexes it resolves the linux/amd64 platform digest
// so that it can be compared against the running container's imageID, while
// also preserving the index digest for display.
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

	if isIndex(desc.MediaType) {
		return c.resolveFromIndex(desc, opts)
	}

	return Result{Digest: desc.Digest.String(), Platform: "linux/amd64"}
}

// resolveFromIndex extracts the linux/amd64 platform digest from a manifest index.
func (c *Checker) resolveFromIndex(desc *remote.Descriptor, opts []remote.Option) Result {
	indexDigest := desc.Digest.String()

	idx, err := desc.ImageIndex()
	if err != nil {
		// Can't parse as index — fall back to returning the index digest itself.
		return Result{Digest: indexDigest, IndexDigest: indexDigest, Platform: "multi-arch"}
	}

	manifest, err := idx.IndexManifest()
	if err != nil {
		return Result{Digest: indexDigest, IndexDigest: indexDigest, Platform: "multi-arch"}
	}

	// Find the linux/amd64 entry — what OpenShift/Kubernetes actually runs.
	target := v1.Platform{OS: "linux", Architecture: "amd64"}
	for _, m := range manifest.Manifests {
		if m.Platform == nil {
			continue
		}
		if m.Platform.OS == target.OS && m.Platform.Architecture == target.Architecture {
			return Result{
				Digest:      m.Digest.String(),
				IndexDigest: indexDigest,
				Platform:    "linux/amd64",
			}
		}
	}

	// linux/amd64 not found in index — return the index digest with a note.
	// Comparison will likely be unknown but we shouldn't crash.
	platforms := make([]string, 0, len(manifest.Manifests))
	for _, m := range manifest.Manifests {
		if m.Platform != nil {
			platforms = append(platforms, m.Platform.OS+"/"+m.Platform.Architecture)
		}
	}
	return Result{
		Digest:      indexDigest,
		IndexDigest: indexDigest,
		Platform:    "multi-arch (" + strings.Join(platforms, ", ") + ")",
	}
}

// LatestTagResult is the outcome of a tag-listing call.
type LatestTagResult struct {
	Tag    string
	Digest string
	Error  error
}

// LatestTag fetches all tags for the repository referenced by imageRef and
// returns the highest semver tag together with its digest.  Non-semver tags
// (e.g. "latest", "main", "edge") are ignored.  When the highest semver tag
// matches currentTag the result is empty (already on latest).
func (c *Checker) LatestTag(ctx context.Context, imageRef, currentTag string) LatestTagResult {
	ref, err := name.ParseReference(imageRef, name.WeakValidation)
	if err != nil {
		return LatestTagResult{Error: fmt.Errorf("parsing reference %q: %w", imageRef, err)}
	}

	opts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithTransport(c.httpClient.Transport),
	}

	// Use the registry + repository part only (strip the tag/digest).
	repo := ref.Context()

	tags, err := remote.ListWithContext(ctx, repo, opts...)
	if err != nil {
		return LatestTagResult{Error: fmt.Errorf("listing tags for %q: %w", repo.String(), err)}
	}

	best := bestSemver(tags)
	if best == "" || best == currentTag {
		return LatestTagResult{} // nothing to report
	}

	// Resolve the digest of the best tag.
	bestRef := repo.Tag(best)
	result := c.Resolve(ctx, bestRef.String())
	if result.Error != nil {
		// We know the tag exists but couldn't get the digest — still return the tag.
		return LatestTagResult{Tag: best}
	}
	return LatestTagResult{Tag: best, Digest: result.Digest}
}

// semverRE matches tags of the form v?MAJOR[.MINOR[.PATCH]][anything]
var semverRE = regexp.MustCompile(`^v?(\d+)(?:\.(\d+)(?:\.(\d+))?)?`)

type semverTag struct {
	raw   string
	major int
	minor int
	patch int
}

// parseSemver returns a semverTag when the tag looks like a version number,
// otherwise returns ok=false.
func parseSemver(tag string) (semverTag, bool) {
	m := semverRE.FindStringSubmatch(tag)
	if m == nil {
		return semverTag{}, false
	}
	atoi := func(s string) int {
		if s == "" {
			return 0
		}
		n, _ := strconv.Atoi(s)
		return n
	}
	return semverTag{
		raw:   tag,
		major: atoi(m[1]),
		minor: atoi(m[2]),
		patch: atoi(m[3]),
	}, true
}

// bestSemver returns the lexicographically/numerically highest semver tag from
// the provided list, or "" if none qualify.
func bestSemver(tags []string) string {
	var parsed []semverTag
	for _, t := range tags {
		if sv, ok := parseSemver(t); ok {
			parsed = append(parsed, sv)
		}
	}
	if len(parsed) == 0 {
		return ""
	}
	sort.Slice(parsed, func(i, j int) bool {
		a, b := parsed[i], parsed[j]
		if a.major != b.major {
			return a.major > b.major
		}
		if a.minor != b.minor {
			return a.minor > b.minor
		}
		return a.patch > b.patch
	})
	return parsed[0].raw
}

func isIndex(mt types.MediaType) bool {
	s := string(mt)
	return strings.Contains(s, "manifest.list") || strings.Contains(s, "index")
}
