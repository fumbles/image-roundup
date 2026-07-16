package chartrepo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/yamlwrangler/image-roundup/backend/internal/models"
)

const maxIndexBytes = 8 * 1024 * 1024

// EnrichHelmReleases annotates installed Helm releases with latest chart
// versions from configured repository indexes.
func EnrichHelmReleases(ctx context.Context, releases []models.HelmRelease, repos []models.HelmRepository, timeout time.Duration) []models.HelmRelease {
	if len(repos) == 0 {
		return releases
	}

	indexes := make([]loadedIndex, 0, len(repos))
	for _, repo := range repos {
		index, err := fetchIndex(ctx, repo, timeout)
		indexes = append(indexes, loadedIndex{repo: repo, index: index, err: err})
	}

	for i := range releases {
		if releases[i].ChartName == "" || releases[i].ChartVersion == "" {
			continue
		}

		var repoErrors []string
		found := false
		for _, loaded := range indexes {
			if loaded.err != nil {
				repoErrors = append(repoErrors, loaded.repo.Name+": "+loaded.err.Error())
				continue
			}
			entries := loaded.index.Entries[releases[i].ChartName]
			if len(entries) == 0 {
				continue
			}
			found = true
			latest := latestChartVersion(entries)
			if latest.Version == "" {
				continue
			}
			releases[i].RepositoryName = loaded.repo.Name
			releases[i].RepositoryURL = loaded.repo.URL
			releases[i].LatestChartVersion = latest.Version
			releases[i].LatestAppVersion = latest.AppVersion
			releases[i].UpdateAvailable = compareVersions(latest.Version, releases[i].ChartVersion) > 0
			break
		}

		if !found && releases[i].Error == "" {
			if len(repoErrors) > 0 {
				releases[i].Error = strings.Join(repoErrors, "; ")
			} else {
				releases[i].Error = "chart not found in configured Helm repositories"
			}
		}
	}

	return releases
}

type loadedIndex struct {
	repo  models.HelmRepository
	index repoIndex
	err   error
}

type repoIndex struct {
	Entries map[string][]chartVersion `yaml:"entries"`
}

type chartVersion struct {
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

func fetchIndex(ctx context.Context, repo models.HelmRepository, timeout time.Duration) (repoIndex, error) {
	url := strings.TrimRight(repo.URL, "/") + "/index.yaml"
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return repoIndex{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return repoIndex{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return repoIndex{}, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := readIndexBody(resp.Body, resp.ContentLength, url)
	if err != nil {
		return repoIndex{}, err
	}

	var index repoIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return repoIndex{}, err
	}
	if index.Entries == nil {
		index.Entries = map[string][]chartVersion{}
	}
	return index, nil
}

func readIndexBody(body io.Reader, contentLength int64, url string) ([]byte, error) {
	if contentLength > maxIndexBytes {
		return nil, fmt.Errorf("GET %s: index.yaml is %d bytes, limit is %d bytes", url, contentLength, maxIndexBytes)
	}

	limited := io.LimitReader(body, maxIndexBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxIndexBytes {
		return nil, fmt.Errorf("GET %s: index.yaml exceeds %d bytes", url, maxIndexBytes)
	}
	return data, nil
}

func latestChartVersion(entries []chartVersion) chartVersion {
	if len(entries) == 0 {
		return chartVersion{}
	}
	candidates := append([]chartVersion(nil), entries...)
	sort.Slice(candidates, func(i, j int) bool {
		return compareVersions(candidates[i].Version, candidates[j].Version) > 0
	})
	return candidates[0]
}

func compareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	max := len(pa)
	if len(pb) > max {
		max = len(pb)
	}
	for i := 0; i < max; i++ {
		av, bv := 0, 0
		if i < len(pa) {
			av = pa[i]
		}
		if i < len(pb) {
			bv = pb[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func parseVersion(version string) []int {
	version = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(version)), "v")
	fields := strings.FieldsFunc(version, func(r rune) bool {
		return r == '.' || r == '-' || r == '+'
	})
	out := make([]int, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		n, err := strconv.Atoi(field)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
