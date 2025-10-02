// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package service provides business logic for report generation.
package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lazycatapps/trivy/backend/internal/models"
	"github.com/lazycatapps/trivy/backend/internal/pkg/logger"
	"github.com/lazycatapps/trivy/backend/internal/repository"
)

// ReportService defines the interface for report operations.
type ReportService interface {
	// GetReport retrieves a scan report in the specified format.
	// If the format differs from the original, it converts the report.
	GetReport(taskID, format string) ([]byte, string, error)

	// GetReportPath returns the file path for a specific report format.
	GetReportPath(taskID, format string) (string, error)
}

// reportServiceImpl implements ReportService.
type reportServiceImpl struct {
	scanRepo   repository.ScanRepository
	reportsDir string
	logger     logger.Logger
}

// NewReportService creates a new report service instance.
func NewReportService(
	scanRepo repository.ScanRepository,
	reportsDir string,
	logger logger.Logger,
) ReportService {
	return &reportServiceImpl{
		scanRepo:   scanRepo,
		reportsDir: reportsDir,
		logger:     logger,
	}
}

// GetReport retrieves a scan report in the specified format.
func (s *reportServiceImpl) GetReport(taskID, format string) ([]byte, string, error) {
	// Get task to verify it exists and is completed
	task, err := s.scanRepo.GetByID(taskID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return nil, "", fmt.Errorf("task not found")
	}
	if task.Status != models.ScanStatusCompleted {
		return nil, "", fmt.Errorf("task not completed")
	}

	// Determine file extension and MIME type
	ext, mimeType := s.getFileExtension(format)

	// User-specific reports directory: reports/users/{userID}/
	userReportsDir := filepath.Join(s.reportsDir, "reports", "users", task.UserID)

	// Check if report exists in the requested format
	reportPath := filepath.Join(userReportsDir, fmt.Sprintf("%s.%s", taskID, ext))

	// If report exists, return it
	if _, err := os.Stat(reportPath); err == nil {
		data, err := os.ReadFile(reportPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read report: %w", err)
		}
		return data, mimeType, nil
	}

	// Report doesn't exist in requested format, need to convert
	s.logger.Info("Converting report for task %s to format %s", taskID, format)

	// Get original report path
	originalExt, _ := s.getFileExtension(task.ScanConfig.Format)
	originalPath := filepath.Join(userReportsDir, fmt.Sprintf("%s.%s", taskID, originalExt))

	// Check if original report exists
	if _, err := os.Stat(originalPath); err != nil {
		// Original report not found, try to use task result data
		if task.Result != nil && task.Result.Data != "" {
			// Save original data first
			if err := os.WriteFile(originalPath, []byte(task.Result.Data), 0644); err != nil {
				return nil, "", fmt.Errorf("failed to save original report: %w", err)
			}
		} else {
			return nil, "", fmt.Errorf("original report not found")
		}
	}

	// Convert report using trivy convert command
	convertedData, err := s.convertReport(originalPath, format)
	if err != nil {
		return nil, "", fmt.Errorf("failed to convert report: %w", err)
	}

	// Save converted report
	if err := os.WriteFile(reportPath, convertedData, 0644); err != nil {
		s.logger.Error("Failed to save converted report: %v", err)
		// Don't fail, just return the data without saving
	}

	return convertedData, mimeType, nil
}

// GetReportPath returns the file path for a specific report format.
func (s *reportServiceImpl) GetReportPath(taskID, format string) (string, error) {
	// Get task to retrieve userID
	task, err := s.scanRepo.GetByID(taskID)
	if err != nil {
		return "", fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return "", fmt.Errorf("task not found")
	}

	ext, _ := s.getFileExtension(format)
	reportPath := filepath.Join(s.reportsDir, "reports", "users", task.UserID, fmt.Sprintf("%s.%s", taskID, ext))

	// Check if report exists
	if _, err := os.Stat(reportPath); err != nil {
		return "", fmt.Errorf("report not found")
	}

	return reportPath, nil
}

// convertReport converts a trivy report from one format to another using trivy convert.
func (s *reportServiceImpl) convertReport(inputPath, targetFormat string) ([]byte, error) {
	// Create temporary output file
	tmpFile, err := os.CreateTemp("", "trivy-report-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Build trivy convert command
	args := []string{
		"convert",
		"--format", targetFormat,
		"--output", tmpPath,
		inputPath,
	}

	s.logger.Info("Executing: trivy %v", args)

	// Execute conversion with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "trivy", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("trivy convert failed: %w (output: %s)", err, string(output))
	}

	// Read converted data
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted report: %w", err)
	}

	return data, nil
}

// getFileExtension returns the file extension and MIME type for a format.
func (s *reportServiceImpl) getFileExtension(format string) (string, string) {
	switch format {
	case "json":
		return "json", "application/json"
	case "table":
		return "txt", "text/plain"
	case "sarif":
		return "sarif", "application/sarif+json"
	case "cyclonedx":
		return "cyclonedx.json", "application/vnd.cyclonedx+json"
	case "spdx":
		return "spdx.json", "application/spdx+json"
	case "html":
		return "html", "text/html"
	default:
		return "txt", "text/plain"
	}
}
