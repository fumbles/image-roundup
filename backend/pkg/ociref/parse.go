// Package ociref provides helpers for parsing OCI image references.
package ociref

import (
	"strings"
)

// Ref is a parsed OCI image reference.
type Ref struct {
	Registry   string
	Repository string
	Tag        string
	Digest     string
}

// defaultRegistry is used when no registry hostname is present.
const defaultRegistry = "docker.io"

// Parse splits an OCI image reference into its components.
// Inputs like the following are accepted:
//
//	ubuntu                           → docker.io / library/ubuntu       : latest
//	ubuntu:22.04                     → docker.io / library/ubuntu       : 22.04
//	quay.io/example/app:v1           → quay.io   / example/app          : v1
//	ghcr.io/org/repo@sha256:abc      → ghcr.io   / org/repo             : "" / sha256:abc
//	localhost:5000/myimage:latest    → localhost:5000 / myimage          : latest
func Parse(image string) Ref {
	var r Ref

	// Split digest
	if idx := strings.Index(image, "@"); idx != -1 {
		r.Digest = image[idx+1:]
		image = image[:idx]
	}

	// Split tag
	// Find last colon, but only if it is after the last slash (avoids port confusion).
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		r.Tag = image[lastColon+1:]
		image = image[:lastColon]
	}

	// Determine if first path component looks like a registry hostname.
	parts := strings.SplitN(image, "/", 2)
	if len(parts) == 2 && isHostname(parts[0]) {
		r.Registry = parts[0]
		r.Repository = parts[1]
	} else {
		r.Registry = defaultRegistry
		if len(parts) == 1 {
			// bare image name like "ubuntu"
			r.Repository = "library/" + parts[0]
		} else {
			r.Repository = image
		}
	}

	if r.Tag == "" && r.Digest == "" {
		r.Tag = "latest"
	}

	return r
}

// isHostname returns true if s looks like a registry hostname
// (contains a dot, a colon/port, or is "localhost").
func isHostname(s string) bool {
	return strings.ContainsAny(s, ".:") || s == "localhost"
}
