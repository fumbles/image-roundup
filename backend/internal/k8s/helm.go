package k8s

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/yamlwrangler/image-roundup/backend/internal/models"
)

// DiscoverHelmReleases lists Helm v3 release secrets and decodes the release
// metadata needed for the Helm page.
func (c *Client) DiscoverHelmReleases(ctx context.Context, opts DiscoveryOptions) ([]models.HelmRelease, error) {
	secrets, err := c.listHelmReleaseSecrets(ctx, opts)
	if err != nil {
		return nil, err
	}

	latest := make(map[string]*corev1.Secret)
	for i := range secrets {
		secret := &secrets[i]
		if isNamespaceExcluded(secret.Namespace, opts) {
			continue
		}
		name := secret.Labels["name"]
		if name == "" {
			name = releaseNameFromSecretName(secret.Name)
		}
		key := secret.Namespace + "/" + name
		current, ok := latest[key]
		if !ok || helmSecretRevision(secret) > helmSecretRevision(current) {
			latest[key] = secret
		}
	}

	releases := make([]models.HelmRelease, 0, len(latest))
	for _, secret := range latest {
		releases = append(releases, helmReleaseFromSecret(secret))
	}

	sort.Slice(releases, func(i, j int) bool {
		if releases[i].Namespace != releases[j].Namespace {
			return releases[i].Namespace < releases[j].Namespace
		}
		return releases[i].Name < releases[j].Name
	})
	return releases, nil
}

func (c *Client) listHelmReleaseSecrets(ctx context.Context, opts DiscoveryOptions) ([]corev1.Secret, error) {
	if len(opts.IncludedNamespaces) == 0 {
		list, err := c.kc.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
			LabelSelector: "owner=helm",
		})
		if err != nil {
			return nil, fmt.Errorf("listing Helm release secrets across namespaces: %w", err)
		}
		return list.Items, nil
	}

	var all []corev1.Secret
	var errs []string
	for _, namespace := range opts.IncludedNamespaces {
		if isNamespaceExcluded(namespace, opts) {
			continue
		}
		list, err := c.kc.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "owner=helm",
		})
		if err != nil {
			errs = append(errs, namespace+": "+err.Error())
			continue
		}
		all = append(all, list.Items...)
	}
	if len(all) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("listing Helm release secrets: %s", strings.Join(errs, "; "))
	}
	return all, nil
}

func (c *Client) discoveryNamespaces(ctx context.Context, opts DiscoveryOptions) ([]string, error) {
	if len(opts.IncludedNamespaces) > 0 {
		return opts.IncludedNamespaces, nil
	}

	nsList, err := c.kc.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}
	namespaces := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

func helmReleaseFromSecret(secret *corev1.Secret) models.HelmRelease {
	release := models.HelmRelease{
		ID:        secret.Namespace + "/" + secret.Labels["name"],
		Name:      secret.Labels["name"],
		Namespace: secret.Namespace,
		Revision:  helmSecretRevision(secret),
		Status:    secret.Labels["status"],
		Updated:   timePtr(secret.CreationTimestamp.Time),
	}
	if release.Name == "" {
		release.Name = releaseNameFromSecretName(secret.Name)
		release.ID = secret.Namespace + "/" + release.Name
	}

	payload, err := decodeHelmReleasePayload(secret.Data["release"])
	if err != nil {
		release.Error = err.Error()
		return release
	}

	if payload.Name != "" {
		release.Name = payload.Name
		release.ID = secret.Namespace + "/" + release.Name
	}
	if payload.Namespace != "" {
		release.Namespace = payload.Namespace
	}
	release.Revision = payload.Version
	if payload.Info.Status != "" {
		release.Status = payload.Info.Status
	}
	if !payload.Info.LastDeployed.IsZero() {
		release.Updated = &payload.Info.LastDeployed
	}
	release.ChartName = payload.Chart.Metadata.Name
	release.ChartVersion = payload.Chart.Metadata.Version
	release.AppVersion = payload.Chart.Metadata.AppVersion
	return release
}

func helmSecretRevision(secret *corev1.Secret) int {
	if version := secret.Labels["version"]; version != "" {
		if n, err := strconv.Atoi(version); err == nil {
			return n
		}
	}
	if idx := strings.LastIndex(secret.Name, ".v"); idx >= 0 {
		if n, err := strconv.Atoi(secret.Name[idx+2:]); err == nil {
			return n
		}
	}
	return 0
}

type helmReleasePayload struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   int    `json:"version"`
	Info      struct {
		Status       string    `json:"status"`
		LastDeployed time.Time `json:"last_deployed"`
	} `json:"info"`
	Chart struct {
		Metadata struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			AppVersion string `json:"appVersion"`
		} `json:"metadata"`
	} `json:"chart"`
}

func decodeHelmReleasePayload(data []byte) (helmReleasePayload, error) {
	if len(data) == 0 {
		return helmReleasePayload{}, fmt.Errorf("missing Helm release payload")
	}

	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return helmReleasePayload{}, fmt.Errorf("decoding Helm release payload: %w", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return helmReleasePayload{}, fmt.Errorf("opening Helm release payload: %w", err)
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return helmReleasePayload{}, fmt.Errorf("reading Helm release payload: %w", err)
	}

	var payload helmReleasePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return helmReleasePayload{}, fmt.Errorf("parsing Helm release payload: %w", err)
	}
	return payload, nil
}

func releaseNameFromSecretName(secretName string) string {
	const prefix = "sh.helm.release.v1."
	name := strings.TrimPrefix(secretName, prefix)
	if idx := strings.LastIndex(name, ".v"); idx > 0 {
		return name[:idx]
	}
	return name
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
