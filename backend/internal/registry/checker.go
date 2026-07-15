// Package registry resolves OCI image digests from remote registries.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	return c.ResolveWithAuth(ctx, imageRef, nil)
}

// ResolveWithAuth is Resolve with an optional explicit authenticator.
func (c *Checker) ResolveWithAuth(ctx context.Context, imageRef string, auth authn.Authenticator) Result {
	ref, err := name.ParseReference(imageRef, name.WeakValidation)
	if err != nil {
		return Result{Error: fmt.Errorf("parsing reference %q: %w", imageRef, err)}
	}

	opts := c.remoteOptions(ctx, auth)

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
	return c.LatestTagWithAuth(ctx, imageRef, currentTag, platform, nil)
}

// LatestTagWithAuth is LatestTag with an optional explicit authenticator.
func (c *Checker) LatestTagWithAuth(ctx context.Context, imageRef, currentTag, platform string, auth authn.Authenticator) LatestTagResult {
	ref, err := name.ParseReference(imageRef, name.WeakValidation)
	if err != nil {
		return LatestTagResult{Error: fmt.Errorf("parsing reference %q: %w", imageRef, err)}
	}

	opts := c.remoteOptions(ctx, auth)

	// Use the registry + repository part only (strip the tag/digest).
	repo := ref.Context()

	tags, err := remote.ListWithContext(ctx, repo, opts...)
	if err != nil {
		return LatestTagResult{Error: fmt.Errorf("listing tags for %q: %w", repo.String(), err)}
	}

	best := selectLatestSemverTag(tags, currentTag, platform, repo.String())
	if best == "" || best == currentTag {
		return LatestTagResult{} // nothing to report
	}

	// Resolve the digest of the best tag.
	bestRef := repo.Tag(best)
	result := c.ResolveWithAuth(ctx, bestRef.String(), auth)
	if result.Error != nil {
		// We know the tag exists but couldn't get the digest — still return the tag.
		return LatestTagResult{Tag: best}
	}
	return LatestTagResult{Tag: best, Digest: result.Digest}
}

func (c *Checker) remoteOptions(ctx context.Context, auth authn.Authenticator) []remote.Option {
	opts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithTransport(c.httpClient.Transport),
	}
	if auth != nil {
		opts = append(opts, remote.WithAuth(auth))
	} else {
		opts = append(opts, remote.WithAuthFromKeychain(dockerHubAliasKeychain{}))
	}
	return opts
}

type dockerHubAliasKeychain struct{}

func (dockerHubAliasKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	auth, err := authn.DefaultKeychain.Resolve(target)
	if err != nil || auth != authn.Anonymous {
		return auth, err
	}
	if target.RegistryStr() != name.DefaultRegistry {
		return auth, nil
	}

	cfg, ok := dockerHubAuthConfig()
	if !ok {
		return auth, nil
	}
	return authn.FromConfig(cfg), nil
}

func dockerHubAuthConfig() (authn.AuthConfig, bool) {
	path := dockerConfigPath()
	if path == "" {
		return authn.AuthConfig{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return authn.AuthConfig{}, false
	}

	var cfg struct {
		Auths map[string]authn.AuthConfig `json:"auths"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return authn.AuthConfig{}, false
	}

	for _, key := range []string{"docker.io", "index.docker.io", authn.DefaultAuthKey} {
		authCfg, ok := cfg.Auths[key]
		if !ok || emptyAuthConfig(authCfg) {
			continue
		}
		return authCfg, true
	}
	return authn.AuthConfig{}, false
}

func dockerConfigPath() string {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".docker", "config.json")
}

func emptyAuthConfig(cfg authn.AuthConfig) bool {
	return cfg.Username == "" &&
		cfg.Password == "" &&
		cfg.Auth == "" &&
		cfg.IdentityToken == "" &&
		cfg.RegistryToken == ""
}

func selectLatestSemverTag(tags []string, currentTag, platform, repository string) string {
	// Derive the arch string to filter on (e.g. "amd64" from "linux/amd64").
	archHint := archFromPlatform(platform)
	compatible := filterByTagCompatibility(filterByArch(tags, currentTag, archHint), currentTag, platform)
	compatible = filterByStreamCompatibility(compatible, currentTag, repository)
	compatible = filterByMajorCompatibility(compatible, currentTag, repository)
	best := bestSemver(compatible)
	if best == "" || best == currentTag {
		return ""
	}

	current, currentOK := parseSemver(currentTag)
	candidate, candidateOK := parseSemver(best)
	if currentOK && candidateOK && !semverGreater(candidate, current) {
		return ""
	}

	return best
}

func filterByStreamCompatibility(tags []string, currentTag, repository string) []string {
	if !linuxServerRepository(repository) {
		return tags
	}

	stream := linuxServerStream(currentTag)
	if stream == "" {
		return tags
	}

	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if linuxServerStream(tag) == stream {
			out = append(out, tag)
		}
	}
	return out
}

func linuxServerRepository(repository string) bool {
	normalized := strings.TrimPrefix(repository, "index.docker.io/")
	normalized = strings.TrimPrefix(normalized, "docker.io/")
	normalized = strings.TrimPrefix(normalized, "lscr.io/")
	return strings.HasPrefix(normalized, "linuxserver/")
}

func linuxServerStream(tag string) string {
	lower := strings.ToLower(tag)
	switch {
	case lower == "latest" || strings.HasSuffix(lower, "-latest"):
		return "latest"
	case lower == "develop" || strings.Contains(lower, "develop"):
		return "develop"
	case lower == "nightly" || strings.Contains(lower, "nightly"):
		return "nightly"
	default:
		return ""
	}
}

func filterByMajorCompatibility(tags []string, currentTag, repository string) []string {
	current, ok := parseSemver(currentTag)
	if !ok || !majorLockedRepository(repository) {
		return tags
	}

	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		candidate, ok := parseSemver(tag)
		if !ok || candidate.major == current.major {
			out = append(out, tag)
		}
	}
	return out
}

func majorLockedRepository(repository string) bool {
	normalized := strings.TrimPrefix(repository, "index.docker.io/")
	normalized = strings.TrimPrefix(normalized, "docker.io/")
	parts := strings.Split(normalized, "/")
	name := parts[len(parts)-1]
	return name == "postgres" || name == "postgresql"
}

func filterByTagCompatibility(tags []string, currentTag, platform string) []string {
	currentVariants := tagVariantTokens(currentTag)
	currentPrerelease := isPrereleaseTag(currentTag)

	out := make([]string, 0, len(tags))
	for _, t := range tags {
		candidateVariants := tagVariantTokens(t)
		if strings.HasPrefix(platform, "linux/") && hasAnyVariant(candidateVariants, windowsVariantTokens) {
			continue
		}
		if !currentPrerelease && isPrereleaseTag(t) {
			continue
		}
		if !variantsCompatible(currentVariants, candidateVariants) {
			continue
		}
		out = append(out, t)
	}
	return out
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

var knownVariantTokens = []string{
	"slim", "alpine", "bookworm", "bullseye", "buster", "trixie",
	"jammy", "noble", "windowsservercore", "nanoserver",
}

var windowsVariantTokens = []string{"windowsservercore", "nanoserver"}

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

var tagTokenRE = regexp.MustCompile(`[a-z0-9]+`)
var prereleaseTokenRE = regexp.MustCompile(`^(?:a|b|rc)\d+$`)

func tagVariantTokens(tag string) map[string]struct{} {
	tokens := tagTokenRE.FindAllString(strings.ToLower(tag), -1)
	variants := make(map[string]struct{})
	for _, token := range tokens {
		for _, variant := range knownVariantTokens {
			if token == variant || strings.HasPrefix(token, variant) {
				variants[variant] = struct{}{}
			}
		}
	}
	return variants
}

func variantsCompatible(current, candidate map[string]struct{}) bool {
	if len(current) == 0 {
		return len(candidate) == 0
	}
	for variant := range current {
		if _, ok := candidate[variant]; !ok {
			return false
		}
	}
	return true
}

func hasAnyVariant(variants map[string]struct{}, candidates []string) bool {
	for _, candidate := range candidates {
		if _, ok := variants[candidate]; ok {
			return true
		}
	}
	return false
}

func isPrereleaseTag(tag string) bool {
	rest := versionRemainder(tag)
	if rest == "" {
		return false
	}
	first := rest[0]
	if first >= 'a' && first <= 'z' {
		return true
	}

	for _, token := range tagTokenRE.FindAllString(strings.ToLower(rest), -1) {
		switch {
		case token == "a", token == "alpha", token == "b", token == "beta", token == "rc":
			return true
		case prereleaseTokenRE.MatchString(token):
			return true
		}
	}
	return false
}

func versionRemainder(tag string) string {
	match := semverRE.FindStringIndex(strings.ToLower(tag))
	if match == nil {
		return ""
	}
	return strings.ToLower(tag[match[1]:])
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

func semverGreater(a, b semverTag) bool {
	if a.major != b.major {
		return a.major > b.major
	}
	if a.minor != b.minor {
		return a.minor > b.minor
	}
	return a.patch > b.patch
}

func isIndex(mt types.MediaType) bool {
	s := string(mt)
	return strings.Contains(s, "manifest.list") || strings.Contains(s, "index")
}
