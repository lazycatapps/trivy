// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package models provides data models for the Trivy Web UI application.
package models

import "time"

// TrivyVersion represents the version information of Trivy Server and its databases.
type TrivyVersion struct {
	Version         string        `json:"version"`
	VulnerabilityDB *DatabaseInfo `json:"vulnerabilityDB,omitempty"`
	JavaDB          *DatabaseInfo `json:"javaDB,omitempty"`
}

// DatabaseInfo represents the metadata of a vulnerability database.
type DatabaseInfo struct {
	Version      int       `json:"version"`
	NextUpdate   time.Time `json:"nextUpdate"`
	UpdatedAt    time.Time `json:"updatedAt"`
	DownloadedAt time.Time `json:"downloadedAt"`
}
