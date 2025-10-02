// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package types defines configuration types for the Trivy Web UI application.
package types

// Config represents the complete application configuration.
type Config struct {
	Server  ServerConfig  // HTTP server configuration
	Trivy   TrivyConfig   // Trivy scan configuration
	CORS    CORSConfig    // CORS policy configuration
	Storage StorageConfig // Storage configuration
	OIDC    OIDCConfig    // OIDC authentication configuration
}

// ServerConfig defines HTTP server listening configuration.
type ServerConfig struct {
	Host string // Server listening address (e.g., "0.0.0.0", "127.0.0.1")
	Port int    // Server listening port (e.g., 8080)
}

// TrivyConfig defines Trivy scan configuration.
type TrivyConfig struct {
	ServerURL         string // Trivy server URL (e.g., "http://trivy-server:4954")
	Timeout           int    // Scan operation timeout in seconds (default: 600)
	DefaultRegistry   string // Default image registry prefix (e.g., "docker.io/")
	AllowPasswordSave bool   // Whether to allow saving passwords in config files (default: false)
	MaxConfigSize     int64  // Maximum size of a single config file in bytes (default: 4096)
	MaxConfigFiles    int    // Maximum number of config files per user (default: 1000)
	MaxWorkers        int    // Maximum concurrent scan workers (default: 5)
	ScanRetentionDays int    // Days to retain scan history (default: 90, 0 = forever)
	EnableDockerScan  bool   // Enable Docker socket access for scanning local images (default: false, requires Docker socket mount)
}

// CORSConfig defines Cross-Origin Resource Sharing policy.
type CORSConfig struct {
	AllowedOrigins []string // Allowed origins (e.g., ["*"], ["https://app.example.com"])
}

// StorageConfig defines storage configuration.
type StorageConfig struct {
	ConfigDir  string // Directory for storing configuration files (default: "/configs")
	ReportsDir string // Directory for storing scan reports and results (default: "/lzcapp/reports")
}

// OIDCConfig defines OIDC authentication configuration.
type OIDCConfig struct {
	ClientID     string // OIDC client ID
	ClientSecret string // OIDC client secret
	Issuer       string // OIDC issuer URL
	RedirectURL  string // OIDC redirect URL after authentication
	Enabled      bool   // Whether OIDC authentication is enabled
}
