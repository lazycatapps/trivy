// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package handler provides HTTP request handlers.
package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lazycatapps/trivy/backend/internal/models"
	"github.com/lazycatapps/trivy/backend/internal/pkg/logger"
	"github.com/lazycatapps/trivy/backend/internal/service"
)

// ScanHandler handles scan-related HTTP requests.
type ScanHandler struct {
	scanService service.ScanService
	logger      logger.Logger
}

// NewScanHandler creates a new scan handler instance.
func NewScanHandler(scanService service.ScanService, logger logger.Logger) *ScanHandler {
	return &ScanHandler{
		scanService: scanService,
		logger:      logger,
	}
}

// CreateScan handles POST /api/v1/scan - Create a new scan task.
func (h *ScanHandler) CreateScan(c *gin.Context) {
	// Get user identifier from session (email, userID, etc.)
	userIdentifier := getUserIdentifier(c)

	// Parse request body
	var req models.ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid scan request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Validate image address
	if req.Image == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image is required"})
		return
	}

	// Create scan task
	task, err := h.scanService.CreateScanTask(userIdentifier, &req)
	if err != nil {
		h.logger.Error("Failed to create scan task: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create scan task"})
		return
	}

	h.logger.Info("Created scan task %s for user %s", task.ID, userIdentifier)

	c.JSON(http.StatusOK, gin.H{
		"message": "Scan started",
		"id":      task.ID,
	})
}

// GetScan handles GET /api/v1/scan/:id - Get scan task details.
func (h *ScanHandler) GetScan(c *gin.Context) {
	taskID := c.Param("id")

	// Get task
	task, err := h.scanService.GetTask(taskID)
	if err != nil {
		h.logger.Error("Failed to get task %s: %v", taskID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// TODO: Check if user owns this task (for OIDC multi-tenancy)
	// For now, allow any user to view any task

	c.JSON(http.StatusOK, task)
}

// ListScans handles GET /api/v1/scan - List scan tasks with pagination.
func (h *ScanHandler) ListScans(c *gin.Context) {
	// Get user identifier from session
	userIdentifier := getUserIdentifier(c)

	// Parse query parameters
	var req models.TaskListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Error("Invalid list request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// List tasks
	response, err := h.scanService.ListTasks(userIdentifier, &req)
	if err != nil {
		h.logger.Error("Failed to list tasks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tasks"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// StreamLogs handles GET /api/v1/scan/:id/logs - Stream scan logs via SSE.
func (h *ScanHandler) StreamLogs(c *gin.Context) {
	taskID := c.Param("id")

	// Get task
	task, err := h.scanService.GetTask(taskID)
	if err != nil {
		h.logger.Error("Failed to get task %s: %v", taskID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// TODO: Check if user owns this task

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Create log listener
	logChan := task.AddLogListener()
	defer task.RemoveLogListener(logChan)

	// Send existing logs first
	existingLogs := task.GetLogLines()
	for _, log := range existingLogs {
		c.SSEvent("message", log)
		c.Writer.Flush()
	}

	// Stream new logs
	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			// Client disconnected
			h.logger.Info("Client disconnected from log stream for task %s", taskID)
			return
		case log, ok := <-logChan:
			if !ok {
				// Channel closed, task completed
				h.logger.Info("Log stream closed for task %s", taskID)
				return
			}
			c.SSEvent("message", log)
			c.Writer.Flush()
		}
	}
}

// GetQueueStatus handles GET /api/v1/queue/status - Get queue status.
func (h *ScanHandler) GetQueueStatus(c *gin.Context) {
	// Get user identifier from session
	userIdentifier := getUserIdentifier(c)

	// Get queue status
	status, err := h.scanService.GetQueueStatus(userIdentifier)
	if err != nil {
		h.logger.Error("Failed to get queue status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queue status"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ListDockerImages handles GET /api/v1/docker/images - List Docker images from host.
func (h *ScanHandler) ListDockerImages(c *gin.Context) {
	h.logger.Info("Listing Docker images from host")

	images, err := h.scanService.ListDockerImages()
	if err != nil {
		h.logger.Error("Failed to list Docker images: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list Docker images: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"images": images,
	})
}

// ListDockerContainers handles GET /api/v1/docker/containers - List running Docker containers from host.
func (h *ScanHandler) ListDockerContainers(c *gin.Context) {
	h.logger.Info("Listing Docker containers from host")

	containers, err := h.scanService.ListDockerContainers()
	if err != nil {
		h.logger.Error("Failed to list Docker containers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list Docker containers: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"containers": containers,
	})
}

// DeleteScan handles DELETE /api/v1/scan/:id - Delete scan task.
func (h *ScanHandler) DeleteScan(c *gin.Context) {
	taskID := c.Param("id")

	// Get task first to check ownership
	_, err := h.scanService.GetTask(taskID)
	if err != nil {
		h.logger.Error("Failed to get task %s: %v", taskID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// TODO: Check if user owns this task (for OIDC multi-tenancy)
	// For now, allow any user to delete any task

	// Delete the task
	if err := h.scanService.DeleteTask(taskID); err != nil {
		h.logger.Error("Failed to delete task %s: %v", taskID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task"})
		return
	}

	h.logger.Info("Deleted scan task %s", taskID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Task deleted successfully",
	})
}

// DeleteAllScans handles DELETE /api/v1/scan - Delete all scan tasks for current user.
func (h *ScanHandler) DeleteAllScans(c *gin.Context) {
	// Get user identifier from session
	userIdentifier := getUserIdentifier(c)

	h.logger.Info("Deleting all scan tasks for user %s", userIdentifier)

	// Delete all tasks
	if err := h.scanService.DeleteAllTasks(userIdentifier); err != nil {
		h.logger.Error("Failed to delete all tasks for user %s: %v", userIdentifier, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete all tasks"})
		return
	}

	h.logger.Info("Deleted all scan tasks for user %s", userIdentifier)
	c.JSON(http.StatusOK, gin.H{
		"message": "All tasks deleted successfully",
	})
}
