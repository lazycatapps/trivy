// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package models

// SavedScanConfig represents a user-saved scan configuration.
// Stored as JSON file on the server.
type SavedScanConfig struct {
	ImagePrefix       string   `json:"imagePrefix,omitempty"`       // Default image prefix
	Username          string   `json:"username,omitempty"`          // Registry username (base64 encoded)
	Password          string   `json:"password,omitempty"`          // Registry password (base64 encoded, optional)
	TLSVerify         bool     `json:"tlsVerify"`                   // TLS verification
	Severity          []string `json:"severity,omitempty"`          // Vulnerability severity filter
	IgnoreUnfixed     bool     `json:"ignoreUnfixed"`               // Ignore unfixed vulnerabilities
	Scanners          []string `json:"scanners,omitempty"`          // Scanner types
	DetectionPriority string   `json:"detectionPriority,omitempty"` // Detection priority
	PkgTypes          []string `json:"pkgTypes,omitempty"`          // Package types
	Format            string   `json:"format,omitempty"`            // Output format
}

// ConfigListResponse represents the response for listing saved configurations.
type ConfigListResponse struct {
	Configs []string `json:"configs"` // List of configuration names
}

// DockerImage represents a Docker image from the host machine.
type DockerImage struct {
	Repository string `json:"repository"` // Image repository
	Tag        string `json:"tag"`        // Image tag
	ImageID    string `json:"imageId"`    // Image ID (short hash)
	Created    string `json:"created"`    // Creation time
	Size       string `json:"size"`       // Image size
	FullName   string `json:"fullName"`   // Full image name (repository:tag)
}

// DockerContainer represents a running Docker container from the host machine.
type DockerContainer struct {
	ContainerID   string `json:"containerId"`   // Container ID (short hash)
	ContainerName string `json:"containerName"` // Container name
	Image         string `json:"image"`         // Image name
	Status        string `json:"status"`        // Container status
	Ports         string `json:"ports"`         // Port mappings
	Created       string `json:"created"`       // Creation time
}
