package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	openShiftImageRegistryNamespace = "openshift-image-registry"
	openShiftImageRegistryService   = "image-registry"
	openShiftImageRegistryRoute     = "default-route"
)

var routeGVR = schema.GroupVersionResource{
	Group:    "route.openshift.io",
	Version:  "v1",
	Resource: "routes",
}

// OpenShiftImageRegistryRouteHost returns the externally trusted host for the
// OpenShift integrated image registry route, when that route exists.
func (c *Client) OpenShiftImageRegistryRouteHost(ctx context.Context) (string, error) {
	routes := c.dyn.Resource(routeGVR).Namespace(openShiftImageRegistryNamespace)

	route, err := routes.Get(ctx, openShiftImageRegistryRoute, metav1.GetOptions{})
	if err == nil {
		if host := routeHost(route); host != "" {
			return host, nil
		}
		return "", fmt.Errorf("route %s/%s has no spec.host", openShiftImageRegistryNamespace, openShiftImageRegistryRoute)
	}

	list, listErr := routes.List(ctx, metav1.ListOptions{})
	if listErr != nil {
		return "", fmt.Errorf("getting route %s/%s: %w; listing fallback routes: %v", openShiftImageRegistryNamespace, openShiftImageRegistryRoute, err, listErr)
	}

	for i := range list.Items {
		route := &list.Items[i]
		if routeServiceName(route) != openShiftImageRegistryService {
			continue
		}
		if host := routeHost(route); host != "" {
			return host, nil
		}
	}

	return "", fmt.Errorf("no route to service %s found in namespace %s", openShiftImageRegistryService, openShiftImageRegistryNamespace)
}

func routeHost(route *unstructured.Unstructured) string {
	host, _, _ := unstructured.NestedString(route.Object, "spec", "host")
	return host
}

func routeServiceName(route *unstructured.Unstructured) string {
	name, _, _ := unstructured.NestedString(route.Object, "spec", "to", "name")
	return name
}
