// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package handler provides HTTP request handlers.
package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lazycatapps/trivy/backend/internal/pkg/logger"
	"github.com/lazycatapps/trivy/backend/internal/service"
)

// ReportHandler handles report download HTTP requests.
type ReportHandler struct {
	reportService service.ReportService
	logger        logger.Logger
}

// NewReportHandler creates a new report handler instance.
func NewReportHandler(reportService service.ReportService, logger logger.Logger) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
		logger:        logger,
	}
}

// DownloadReport handles GET /api/v1/scan/:id/report/:format - Download scan report.
func (h *ReportHandler) DownloadReport(c *gin.Context) {
	taskID := c.Param("id")
	format := c.Param("format")

	// Validate format
	validFormats := map[string]bool{
		"json":      true,
		"html":      true,
		"sarif":     true,
		"cyclonedx": true,
		"spdx":      true,
		"table":     true,
	}

	if !validFormats[format] {
		h.logger.Error("Invalid report format: %s", format)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported format: %s", format)})
		return
	}

	// Get report
	data, mimeType, err := h.reportService.GetReport(taskID, format)
	if err != nil {
		h.logger.Error("Failed to get report for task %s (format: %s): %v", taskID, format, err)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		} else if strings.Contains(err.Error(), "not completed") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Scan not completed"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate report"})
		}
		return
	}

	// Generate filename
	timestamp := time.Now().Format("20060102-150405")
	ext := format
	if format == "cyclonedx" || format == "spdx" {
		ext = format + ".json"
	} else if format == "sarif" {
		ext = "sarif"
	} else if format == "table" {
		ext = "txt"
	}
	filename := fmt.Sprintf("trivy-report-%s-%s.%s", taskID[:8], timestamp, ext)

	h.logger.Info("Serving report for task %s (format: %s, size: %d bytes)", taskID, format, len(data))

	// Set response headers
	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Length", fmt.Sprintf("%d", len(data)))

	// Send file
	c.Data(http.StatusOK, mimeType, data)
}
