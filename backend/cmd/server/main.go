// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package main is the entry point for the Trivy Web UI server application.
// It initializes all dependencies, configures the server, and starts the HTTP service.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lazycatapps/trivy/backend/internal/handler"
	"github.com/lazycatapps/trivy/backend/internal/pkg/logger"
	"github.com/lazycatapps/trivy/backend/internal/repository"
	"github.com/lazycatapps/trivy/backend/internal/router"
	"github.com/lazycatapps/trivy/backend/internal/service"
	"github.com/lazycatapps/trivy/backend/internal/types"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd is the root command for the CLI application.
var rootCmd = &cobra.Command{
	Use:   "trivy-web",
	Short: "Trivy Web UI - Container security scanning with web interface",
	Long:  `A web service for scanning container images using Aqua Security Trivy.`,
	Run:   runServer,
}

// init initializes command-line flags and environment variable bindings.
func init() {
	rootCmd.Flags().String("host", "0.0.0.0", "Server host")
	rootCmd.Flags().IntP("port", "p", 8080, "Server port")
	rootCmd.Flags().Int("timeout", 600, "Scan timeout in seconds")
	rootCmd.Flags().String("trivy-server", "", "Trivy server URL (e.g., http://trivy-server:4954)")
	rootCmd.Flags().String("default-registry", "docker.io/", "Default image registry prefix")
	rootCmd.Flags().StringSlice("cors-allowed-origins", []string{"*"}, "CORS allowed origins")
	rootCmd.Flags().String("config-dir", "./configs", "Directory for storing configuration files")
	rootCmd.Flags().String("reports-dir", "./reports", "Directory for storing scan reports and results")
	rootCmd.Flags().Bool("allow-password-save", false, "Allow saving passwords in configuration files")
	rootCmd.Flags().Int64("max-config-size", 4096, "Maximum configuration file size in bytes")
	rootCmd.Flags().Int("max-config-files", 1000, "Maximum number of configuration files per user")
	rootCmd.Flags().Int("max-workers", 5, "Maximum concurrent scan workers")
	rootCmd.Flags().Int("scan-retention-days", 90, "Days to retain scan history (0 = forever)")
	rootCmd.Flags().String("oidc-client-id", "", "OIDC client ID")
	rootCmd.Flags().String("oidc-client-secret", "", "OIDC client secret")
	rootCmd.Flags().String("oidc-issuer", "", "OIDC issuer URL")
	rootCmd.Flags().String("oidc-redirect-url", "", "OIDC redirect URL")
	rootCmd.Flags().Bool("enable-docker-scan", false, "Enable Docker socket access for scanning local images (requires Docker socket mount)")

	viper.BindPFlags(rootCmd.Flags())

	// Set environment variable prefix to "TRIVY"
	viper.SetEnvPrefix("TRIVY")
	viper.AutomaticEnv()
	// Replace hyphens with underscores in environment variable names
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
}

// runServer is the main server execution function.
func runServer(cmd *cobra.Command, args []string) {
	// Load configuration from viper
	oidcClientID := viper.GetString("oidc-client-id")
	oidcClientSecret := viper.GetString("oidc-client-secret")
	oidcIssuer := viper.GetString("oidc-issuer")
	oidcRedirectURL := viper.GetString("oidc-redirect-url")

	cfg := &types.Config{
		Server: types.ServerConfig{
			Host: viper.GetString("host"),
			Port: viper.GetInt("port"),
		},
		Trivy: types.TrivyConfig{
			ServerURL:         viper.GetString("trivy-server"),
			Timeout:           viper.GetInt("timeout"),
			DefaultRegistry:   viper.GetString("default-registry"),
			AllowPasswordSave: viper.GetBool("allow-password-save"),
			MaxConfigSize:     viper.GetInt64("max-config-size"),
			MaxConfigFiles:    viper.GetInt("max-config-files"),
			MaxWorkers:        viper.GetInt("max-workers"),
			ScanRetentionDays: viper.GetInt("scan-retention-days"),
			EnableDockerScan:  viper.GetBool("enable-docker-scan"),
		},
		CORS: types.CORSConfig{
			AllowedOrigins: viper.GetStringSlice("cors-allowed-origins"),
		},
		Storage: types.StorageConfig{
			ConfigDir:  viper.GetString("config-dir"),
			ReportsDir: viper.GetString("reports-dir"),
		},
		OIDC: types.OIDCConfig{
			ClientID:     oidcClientID,
			ClientSecret: oidcClientSecret,
			Issuer:       oidcIssuer,
			RedirectURL:  oidcRedirectURL,
			Enabled:      oidcClientID != "" && oidcClientSecret != "" && oidcIssuer != "",
		},
	}

	// Initialize logger
	log := logger.New()

	log.Info("Starting Trivy Web UI server")
	log.Info("=================================")

	// Log Trivy configuration
	log.Info("Trivy Configuration:")
	log.Info("  Server URL: %s", cfg.Trivy.ServerURL)
	log.Info("  Timeout: %d seconds", cfg.Trivy.Timeout)
	log.Info("  Max Workers: %d", cfg.Trivy.MaxWorkers)
	log.Info("  Scan Retention: %d days", cfg.Trivy.ScanRetentionDays)
	log.Info("  Allow Password Save: %v", cfg.Trivy.AllowPasswordSave)
	log.Info("  Enable Docker Scan: %v", cfg.Trivy.EnableDockerScan)

	// Log OIDC configuration status
	if cfg.OIDC.Enabled {
		log.Info("OIDC authentication: ENABLED")
		log.Info("  Issuer: %s", cfg.OIDC.Issuer)
		log.Info("  Client ID: %s", cfg.OIDC.ClientID)
		log.Info("  Redirect URL: %s", cfg.OIDC.RedirectURL)
	} else {
		log.Info("OIDC authentication: DISABLED")
	}

	// Initialize repository (file-based task storage with persistence)
	log.Info("Initializing scan repository...")
	log.Info("  Reports directory: %s", cfg.Storage.ReportsDir)
	scanRepo, err := repository.NewFileBasedScanRepository(cfg.Storage.ReportsDir)
	if err != nil {
		log.Error("Failed to initialize scan repository: %v", err)
		return
	}
	log.Info("Scan repository initialized successfully")

	// Initialize services
	scanService := service.NewScanService(scanRepo, &cfg.Trivy, cfg.Storage.ReportsDir, log)
	reportService := service.NewReportService(scanRepo, cfg.Storage.ReportsDir, log)
	configService := service.NewConfigService(
		cfg.Storage.ConfigDir,
		cfg.Trivy.AllowPasswordSave,
		int(cfg.Trivy.MaxConfigSize),
		cfg.Trivy.MaxConfigFiles,
		log,
	)
	sessionService := service.NewSessionService(7 * 24 * time.Hour) // 7 days session TTL

	// Start scan service worker pool
	scanService.Start()
	defer scanService.Stop()

	// Initialize HTTP handlers
	scanHandler := handler.NewScanHandler(scanService, log)
	reportHandler := handler.NewReportHandler(reportService, log)
	configHandler := handler.NewConfigHandler(configService, cfg.Trivy.EnableDockerScan, log)

	// Initialize auth handler
	authHandler, err := handler.NewAuthHandler(&cfg.OIDC, sessionService, log)
	if err != nil {
		log.Error("Failed to initialize auth handler: %v", err)
		return
	}

	// Set up router and middleware
	r := router.New(scanHandler, reportHandler, configHandler, authHandler, sessionService)
	engine := r.Setup(cfg)

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server in goroutine
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info("=================================")
	log.Info("Server listening on %s", addr)
	log.Info("Press Ctrl+C to stop")

	go func() {
		if err := engine.Run(addr); err != nil {
			log.Error("Server failed: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Info("Shutting down server...")
	log.Info("Goodbye!")
}

// main is the application entry point.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
