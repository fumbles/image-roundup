// Package k8s discovers running container images from Kubernetes workloads.
package k8s

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/yamlwrangler/image-roundup/backend/internal/models"
	"github.com/yamlwrangler/image-roundup/backend/pkg/ociref"
)

// Client wraps the Kubernetes client and discovery logic.
type Client struct {
	kc  kubernetes.Interface
	log *zap.Logger
}

// New creates a Client using in-cluster config when inCluster is true,
// otherwise falling back to kubeConfigPath.
func New(inCluster bool, kubeConfigPath string, log *zap.Logger) (*Client, error) {
	var cfg *rest.Config
	var err error
	if inCluster {
		cfg, err = rest.InClusterConfig()
	} else {
		if kubeConfigPath == "" {
			kubeConfigPath = clientcmd.RecommendedHomeFile
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}
	if err != nil {
		return nil, fmt.Errorf("building kube config: %w", err)
	}
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building kube client: %w", err)
	}
	return &Client{kc: kc, log: log}, nil
}

// DiscoverImages returns a deduplicated list of ImageRecords found across all
// accessible namespaces, filtered according to opts.
func (c *Client) DiscoverImages(ctx context.Context, opts DiscoveryOptions) ([]*models.ImageRecord, error) {
	// Collect pods
	pods, err := c.listPods(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Build image records indexed by composite key.
	byKey := make(map[string]*models.ImageRecord)

	for _, pod := range pods {
		if opts.SkipCompleted && isPodCompleted(pod) {
			continue
		}
		if isNamespaceExcluded(pod.Namespace, opts) {
			continue
		}

		owner := podOwner(pod)
		for _, cs := range pod.Status.ContainerStatuses {
			specImage := containerSpecImage(pod, cs.Name)
			if specImage == "" {
				specImage = cs.Image
			}

			ref := ociref.Parse(specImage)
			key := fmt.Sprintf("%s:%s:%s:%s", pod.Namespace, owner.Kind, owner.Name, cs.Name)

			rec, exists := byKey[key]
			if !exists {
				rec = &models.ImageRecord{
					ID:              key,
					Namespace:       pod.Namespace,
					WorkloadKind:    owner.Kind,
					WorkloadName:    owner.Name,
					ContainerName:   cs.Name,
					ConfiguredImage: specImage,
					Registry:        ref.Registry,
					Repository:      ref.Repository,
					Tag:             ref.Tag,
					Status:          models.StatusUnknown,
					PodNames:        []string{},
				}
				byKey[key] = rec
			}

			// Always collect pod names
			rec.PodNames = appendUnique(rec.PodNames, pod.Name)

			// Extract running digest from container status
			if digest := extractDigest(cs.ImageID); digest != "" {
				rec.RunningDigest = digest
			}
		}

		// Also walk init containers
		for _, cs := range pod.Status.InitContainerStatuses {
			specImage := containerSpecImage(pod, cs.Name)
			if specImage == "" {
				specImage = cs.Image
			}
			ref := ociref.Parse(specImage)
			key := fmt.Sprintf("%s:%s:%s:init:%s", pod.Namespace, owner.Kind, owner.Name, cs.Name)

			rec, exists := byKey[key]
			if !exists {
				rec = &models.ImageRecord{
					ID:              key,
					Namespace:       pod.Namespace,
					WorkloadKind:    owner.Kind,
					WorkloadName:    owner.Name,
					ContainerName:   "init:" + cs.Name,
					ConfiguredImage: specImage,
					Registry:        ref.Registry,
					Repository:      ref.Repository,
					Tag:             ref.Tag,
					Status:          models.StatusUnknown,
					PodNames:        []string{},
				}
				byKey[key] = rec
			}
			rec.PodNames = appendUnique(rec.PodNames, pod.Name)
			if digest := extractDigest(cs.ImageID); digest != "" {
				rec.RunningDigest = digest
			}
		}
	}

	records := make([]*models.ImageRecord, 0, len(byKey))
	for _, r := range byKey {
		records = append(records, r)
	}
	return records, nil
}

// DiscoveryOptions controls which pods are included.
type DiscoveryOptions struct {
	IncludedNamespaces []string
	ExcludedNamespaces []string
	SkipCompleted      bool
}

// listPods returns all pods across included namespaces (or all namespaces).
func (c *Client) listPods(ctx context.Context, opts DiscoveryOptions) ([]corev1.Pod, error) {
	namespaces := opts.IncludedNamespaces
	if len(namespaces) == 0 {
		// List all namespaces
		nsList, err := c.kc.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing namespaces: %w", err)
		}
		for _, ns := range nsList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	var all []corev1.Pod
	for _, ns := range namespaces {
		if isNamespaceExcluded(ns, opts) {
			continue
		}
		list, err := c.kc.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			c.log.Warn("failed to list pods in namespace", zap.String("namespace", ns), zap.Error(err))
			continue
		}
		all = append(all, list.Items...)
	}
	return all, nil
}

// --- helpers ---

type ownerRef struct {
	Kind string
	Name string
}

// podOwner returns the most meaningful owner for a pod.
func podOwner(pod corev1.Pod) ownerRef {
	for _, ref := range pod.OwnerReferences {
		switch ref.Kind {
		case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob", "DeploymentConfig":
			return ownerRef{Kind: ref.Kind, Name: ref.Name}
		}
	}
	return ownerRef{Kind: "Pod", Name: pod.Name}
}

func isPodCompleted(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}

func isNamespaceExcluded(ns string, opts DiscoveryOptions) bool {
	for _, ex := range opts.ExcludedNamespaces {
		if ex == ns {
			return true
		}
	}
	return false
}

// containerSpecImage returns the image configured in the pod spec for the named container.
func containerSpecImage(pod corev1.Pod, name string) string {
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return c.Image
		}
	}
	for _, c := range pod.Spec.InitContainers {
		if c.Name == name {
			return c.Image
		}
	}
	return ""
}

// extractDigest pulls a sha256 digest out of an image ID string.
// ImageID formats: sha256:abc, docker-pullable://registry/repo@sha256:abc
func extractDigest(imageID string) string {
	if idx := strings.Index(imageID, "sha256:"); idx != -1 {
		return imageID[idx:]
	}
	return ""
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
