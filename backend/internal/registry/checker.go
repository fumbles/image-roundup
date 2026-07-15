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
// Registry credentials are picked up automatically from the DOCKER_CONFIG
// environment variable (should point to the directory containing config.json).
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
//
// Architecture-aware: when the running record has a known platform (e.g.
// "linux/amd64"), only tags that match the same architecture suffix (or no
// architecture suffix at all) are considered.  This prevents e.g. an amd64
// workload from being told that an armhf tag is "newer".
func (c *Checker) LatestTag(ctx context.Context, imageRef, currentTag, platform string) LatestTagResult {
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

	// Derive the arch string to filter on (e.g. "amd64" from "linux/amd64").
	archHint := archFromPlatform(platform)
	best := bestSemver(filterByArch(tags, currentTag, archHint))
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

// archFromPlatform extracts the architecture component from a platform string
// like "linux/amd64" → "amd64", "linux/arm64" → "arm64".
// Returns "" when the platform is empty or unparseable.
func archFromPlatform(platform string) string {
	parts := strings.SplitN(platform, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// knownArchSuffixes are the architecture strings that can appear as tag suffixes.
var knownArchSuffixes = []string{
	"amd64", "arm64", "arm", "armhf", "armel",
	"386", "s390x", "ppc64le", "mips64le",
}

// filterByArch filters tags so that only those compatible with the current
// tag's architecture intent are returned.
//
// Rules:
//   - Determine the "arch suffix" of the current tag (if any).
//   - If the current tag ends in a known arch suffix (e.g. "-armhf"), keep only
//     tags that end in the same suffix.
//   - If the current tag has no arch suffix, drop all tags that end in any
//     known arch suffix (they are arch-specific variants, not the generic build).
//   - Non-semver tags are passed through unchanged (bestSemver will ignore them).
func filterByArch(tags []string, currentTag, archHint string) []string {
	currentArchSuffix := tagArchSuffix(currentTag)
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		tagSuffix := tagArchSuffix(t)
		if currentArchSuffix != "" {
			// Current tag is arch-specific — require exact same suffix.
			if tagSuffix == currentArchSuffix {
				out = append(out, t)
			}
		} else {
			// Current tag is not arch-specific — skip any arch-specific variants.
			// Also respect the archHint: if we know we're on amd64, prefer amd64.
			if tagSuffix == "" || (archHint != "" && tagSuffix == archHint) {
				out = append(out, t)
			}
		}
	}
	return out
}

// tagArchSuffix returns the architecture suffix of a tag (e.g. "-armhf" → "armhf",
// "-arm64" → "arm64") or "" if the tag has no recognised arch suffix.
// It checks for suffixes delimited by "-", "." or "_".
func tagArchSuffix(tag string) string {
	lower := strings.ToLower(tag)
	for _, arch := range knownArchSuffixes {
		for _, sep := range []string{"-", ".", "_"} {
			if strings.HasSuffix(lower, sep+arch) {
				return arch
			}
		}
	}
	return ""
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
