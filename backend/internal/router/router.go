// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package router provides HTTP routing configuration for the Trivy Web UI server.
package router

import (
	"github.com/lazycatapps/trivy/backend/internal/handler"
	"github.com/lazycatapps/trivy/backend/internal/middleware"
	"github.com/lazycatapps/trivy/backend/internal/types"

	"github.com/gin-gonic/gin"
)

// Router manages HTTP request routing and handler registration.
// It holds references to all HTTP handlers (scan, report, config, auth, etc.).
type Router struct {
	scanHandler      *handler.ScanHandler
	reportHandler    *handler.ReportHandler
	configHandler    *handler.ConfigHandler
	authHandler      *handler.AuthHandler
	sessionValidator middleware.SessionValidator
}

// New creates a new Router instance with the provided handlers.
func New(
	scanHandler *handler.ScanHandler,
	reportHandler *handler.ReportHandler,
	configHandler *handler.ConfigHandler,
	authHandler *handler.AuthHandler,
	sessionValidator middleware.SessionValidator,
) *Router {
	return &Router{
		scanHandler:      scanHandler,
		reportHandler:    reportHandler,
		configHandler:    configHandler,
		authHandler:      authHandler,
		sessionValidator: sessionValidator,
	}
}

// Setup initializes the Gin engine with middleware and routes.
// It configures the following middleware in order:
//  1. gin.Logger() - HTTP request logging
//  2. gin.Recovery() - Panic recovery
//  3. CORS - Cross-Origin Resource Sharing
//  4. Auth - OIDC authentication (if enabled)
//
// Returns a configured *gin.Engine ready to serve HTTP requests.
func (r *Router) Setup(cfg *types.Config) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	engine.Use(middleware.Auth(cfg.OIDC.Enabled, r.sessionValidator))

	// Disable trusted proxy feature for security
	engine.SetTrustedProxies(nil)

	r.registerRoutes(engine)

	return engine
}

// registerRoutes registers all API routes under /api/v1 prefix.
// Available endpoints:
//   - GET    /health               - Health check
//   - GET    /auth/login           - Redirect to OIDC provider for login
//   - GET    /auth/callback        - OIDC callback handler
//   - POST   /auth/logout          - Logout current user
//   - GET    /auth/userinfo        - Get current user information
//   - POST   /scan                 - Create a new scan task
//   - GET    /scan                 - List scan tasks with pagination and filtering
//   - GET    /scan/:id             - Get scan task status and details
//   - GET    /scan/:id/logs        - Stream scan task logs via SSE
//   - GET    /scan/:id/report/:format - Download scan report in specified format
//   - GET    /queue/status         - Get queue status
//   - GET    /configs              - List all saved configuration names
//   - GET    /config/last-used     - Get the name of the last used configuration
//   - GET    /config/:name         - Get a saved user configuration by name
//   - POST   /config/:name         - Save user configuration with name
//   - DELETE /config/:name         - Delete a saved user configuration by name
func (r *Router) registerRoutes(engine *gin.Engine) {
	api := engine.Group("/api/v1")
	{
		// Public endpoints (no auth required)
		api.GET("/health", r.healthCheck)

		// Auth endpoints
		auth := api.Group("/auth")
		{
			auth.GET("/login", r.authHandler.Login)
			auth.GET("/callback", r.authHandler.Callback)
			auth.POST("/logout", r.authHandler.Logout)
			auth.GET("/userinfo", r.authHandler.UserInfo)
		}

		// Protected endpoints (require auth if OIDC enabled)
		// Scan endpoints
		api.POST("/scan", r.scanHandler.CreateScan)
		api.GET("/scan", r.scanHandler.ListScans)
		api.DELETE("/scan", r.scanHandler.DeleteAllScans)
		api.GET("/scan/:id", r.scanHandler.GetScan)
		api.DELETE("/scan/:id", r.scanHandler.DeleteScan)
		api.GET("/scan/:id/logs", r.scanHandler.StreamLogs)

		// Report download endpoints
		api.GET("/scan/:id/report/:format", r.reportHandler.DownloadReport)

		// Queue status endpoint
		api.GET("/queue/status", r.scanHandler.GetQueueStatus)

		// Docker endpoints
		api.GET("/docker/images", r.scanHandler.ListDockerImages)
		api.GET("/docker/containers", r.scanHandler.ListDockerContainers)

		// Config management endpoints
		api.GET("/configs", r.configHandler.ListConfigs)
		api.GET("/config/last-used", r.configHandler.GetLastUsedConfig)
		api.GET("/config/:name", r.configHandler.GetConfig)
		api.POST("/config/:name", r.configHandler.SaveConfig)
		api.DELETE("/config/:name", r.configHandler.DeleteConfig)

		// System config endpoint (public)
		api.GET("/system/config", r.configHandler.GetSystemConfig)
	}
}

// healthCheck is a simple health check endpoint.
func (r *Router) healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
