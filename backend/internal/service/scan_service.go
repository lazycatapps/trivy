// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package service provides business logic for scan operations.
package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lazycatapps/trivy/backend/internal/models"
	"github.com/lazycatapps/trivy/backend/internal/pkg/logger"
	"github.com/lazycatapps/trivy/backend/internal/repository"
	"github.com/lazycatapps/trivy/backend/internal/types"
)

// CommandExecutor defines the interface for executing external commands.
type CommandExecutor interface {
	// ExecuteCommand executes a command with streaming output callback.
	// The logCallback is called for each line of stdout/stderr output.
	ExecuteCommand(ctx context.Context, name string, args []string, logCallback func(string)) (stdout, stderr string, err error)
}

// realCommandExecutor implements CommandExecutor using exec.CommandContext.
type realCommandExecutor struct{}

// ExecuteCommand executes a real system command with streaming output.
func (e *realCommandExecutor) ExecuteCommand(ctx context.Context, name string, args []string, logCallback func(string)) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	// Capture stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("failed to start command: %w", err)
	}

	// Stream logs from stdout and stderr
	var wg sync.WaitGroup
	var stdout, stderr strings.Builder

	// Pre-allocate capacity for better performance
	stdout.Grow(64 * 1024) // 64KB initial capacity
	stderr.Grow(8 * 1024)  // 8KB initial capacity

	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		// Increase buffer size for long lines (e.g., docker ps JSON output)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024) // Max 1MB per line
		for scanner.Scan() {
			line := scanner.Text()
			stdout.WriteString(line)
			stdout.WriteByte('\n')
			if logCallback != nil {
				logCallback(line)
			}
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			stderr.WriteString(line)
			stderr.WriteByte('\n')
			if logCallback != nil {
				logCallback(line)
			}
		}
	}()

	// Wait for output streams to complete
	wg.Wait()

	// Wait for command to complete
	err = cmd.Wait()
	return stdout.String(), stderr.String(), err
}

// ScanService defines the interface for scan operations.
type ScanService interface {
	// CreateScanTask creates a new scan task and adds it to the queue.
	CreateScanTask(userID string, req *models.ScanRequest) (*models.ScanTask, error)

	// GetTask retrieves a scan task by ID.
	GetTask(taskID string) (*models.ScanTask, error)

	// ListTasks retrieves scan tasks with pagination and filtering.
	ListTasks(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error)

	// GetQueueStatus returns the current queue status for a user.
	GetQueueStatus(userID string) (*models.QueueStatusResponse, error)

	// GetTrivyVersion retrieves version information from Trivy Server.
	GetTrivyVersion(ctx context.Context) (*models.TrivyVersion, error)

	// ListDockerImages lists Docker images from the host machine.
	ListDockerImages() ([]models.DockerImage, error)

	// ListDockerContainers lists running Docker containers from the host machine.
	ListDockerContainers() ([]models.DockerContainer, error)

	// DeleteTask deletes a scan task and its report files.
	DeleteTask(taskID string) error

	// DeleteAllTasks deletes all scan tasks and their report files for a user.
	DeleteAllTasks(userID string) error

	// Start starts the scan worker pool.
	Start()

	// Stop stops the scan worker pool gracefully.
	Stop()
}

// scanServiceImpl implements ScanService.
type scanServiceImpl struct {
	repo       repository.ScanRepository
	config     *types.TrivyConfig
	storageDir string
	logger     logger.Logger
	executor   CommandExecutor // Command executor for trivy

	// Worker pool management
	workerPool chan struct{}  // Semaphore for limiting concurrent scans
	stopCh     chan struct{}  // Signal to stop worker
	wg         sync.WaitGroup // Wait group for graceful shutdown
	mu         sync.Mutex     // Mutex for thread-safe operations
}

// NewScanService creates a new scan service instance.
func NewScanService(
	repo repository.ScanRepository,
	config *types.TrivyConfig,
	storageDir string,
	logger logger.Logger,
) ScanService {
	return NewScanServiceWithExecutor(repo, config, storageDir, logger, &realCommandExecutor{})
}

// NewScanServiceWithExecutor creates a new scan service instance with custom executor.
// This is useful for testing with mock executors.
func NewScanServiceWithExecutor(
	repo repository.ScanRepository,
	config *types.TrivyConfig,
	storageDir string,
	logger logger.Logger,
	executor CommandExecutor,
) ScanService {
	maxWorkers := config.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 5 // Default to 5 concurrent scans
	}

	return &scanServiceImpl{
		repo:       repo,
		config:     config,
		storageDir: storageDir,
		logger:     logger,
		executor:   executor,
		workerPool: make(chan struct{}, maxWorkers),
		stopCh:     make(chan struct{}),
	}
}

// Start starts the scan worker pool.
func (s *scanServiceImpl) Start() {
	s.logger.Info("Starting scan service worker pool (max workers: %d)", cap(s.workerPool))

	// Mark all running tasks as failed (server restart recovery)
	s.markInterruptedTasksAsFailed()

	s.wg.Add(1)
	go s.queueWorker()

	// Start cleanup worker if retention is enabled
	if s.config.ScanRetentionDays > 0 {
		s.logger.Info("Starting cleanup worker (retention: %d days)", s.config.ScanRetentionDays)
		s.wg.Add(1)
		go s.cleanupWorker()
	} else {
		s.logger.Info("Cleanup disabled (retention days = 0, keeping all scans)")
	}
}

// Stop stops the scan worker pool gracefully.
func (s *scanServiceImpl) Stop() {
	s.logger.Info("Stopping scan service worker pool...")
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("Scan service worker pool stopped")
}

// queueWorker processes the scan queue.
func (s *scanServiceImpl) queueWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processQueue()
		}
	}
}

// cleanupWorker periodically cleans up old scan reports.
func (s *scanServiceImpl) cleanupWorker() {
	defer s.wg.Done()

	// Run cleanup every 24 hours at 2 AM local time
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run cleanup immediately on startup
	s.cleanupOldReports()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			// Check if it's 2 AM (run once per day)
			now := time.Now()
			if now.Hour() == 2 {
				s.cleanupOldReports()
			}
		}
	}
}

// processQueue checks for queued tasks and starts scans if workers are available.
func (s *scanServiceImpl) processQueue() {
	// Try to acquire a worker slot (non-blocking)
	select {
	case s.workerPool <- struct{}{}:
		// Worker acquired, find next queued task
		task := s.getNextQueuedTask()
		if task == nil {
			// No queued tasks, release worker
			<-s.workerPool
			return
		}

		// Start scan in goroutine
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.workerPool }() // Release worker when done

			s.executeScan(task)
		}()
	default:
		// All workers busy, skip this tick
	}
}

// getNextQueuedTask retrieves the next queued task across all users (FIFO).
func (s *scanServiceImpl) getNextQueuedTask() *models.ScanTask {
	// Note: This is a simplified implementation.
	// In production, you might want to use a proper queue with priorities.

	// For now, we'll just scan through the repository
	// This is not efficient but works for the MVP
	return nil // Placeholder - will be improved
}

// CreateScanTask creates a new scan task and adds it to the queue.
func (s *scanServiceImpl) CreateScanTask(userID string, req *models.ScanRequest) (*models.ScanTask, error) {
	// Generate unique task ID
	taskID := uuid.New().String()

	// Set default values
	if req.TLSVerify == nil {
		defaultTLS := true
		req.TLSVerify = &defaultTLS
	}
	if len(req.Scanners) == 0 {
		req.Scanners = []string{"vuln"}
	}
	if req.DetectionPriority == "" {
		req.DetectionPriority = "precise"
	}
	if req.Format == "" {
		req.Format = "json"
	}
	// Note: In client-server mode, database configuration is managed by Trivy Server
	// Client does not need --skip-db-update, --db-repository, or --java-db-repository flags

	// Create scan config
	scanConfig := &models.ScanConfig{
		Username:          req.Username,
		Password:          req.Password,
		TLSVerify:         *req.TLSVerify,
		Severity:          req.Severity,
		IgnoreUnfixed:     req.IgnoreUnfixed,
		Scanners:          req.Scanners,
		DetectionPriority: req.DetectionPriority,
		PkgTypes:          req.PkgTypes,
		Format:            req.Format,
	}

	// Create scan task
	task := models.NewScanTask(taskID, userID, req.Image, scanConfig)

	// Save task to repository
	if err := s.repo.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	s.logger.Info("Created scan task %s for user %s (image: %s)", taskID, userID, req.Image)

	// Try to start scan immediately if worker is available
	select {
	case s.workerPool <- struct{}{}:
		// Worker acquired, start scan
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.workerPool }()
			s.executeScan(task)
		}()
	default:
		// All workers busy, task will wait in queue
		s.logger.Info("All workers busy, task %s queued", taskID)
	}

	return task, nil
}

// executeScan executes a Trivy scan for the given task.
func (s *scanServiceImpl) executeScan(task *models.ScanTask) {
	s.logger.Info("Starting scan for task %s (image: %s)", task.ID, task.Image)

	// Update task status to running
	task.Status = models.ScanStatusRunning
	task.Message = "Scan in progress"
	task.AddLog(fmt.Sprintf("Scan started at %s", time.Now().Format(time.RFC3339)))
	s.repo.Update(task)

	// Create context with timeout
	timeout := time.Duration(s.config.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Fetch and record Trivy Server version
	versionCtx, versionCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer versionCancel()
	if version, err := s.GetTrivyVersion(versionCtx); err != nil {
		s.logger.Error("Failed to fetch Trivy Server version: %v", err)
		task.AddLog(fmt.Sprintf("Warning: Could not fetch Trivy Server version: %v", err))
		// Don't fail the scan, just continue without version info
	} else {
		task.TrivyVersion = version
		task.AddLog(fmt.Sprintf("Trivy Server version: %s", version.Version))
		if version.VulnerabilityDB != nil {
			task.AddLog(fmt.Sprintf("Vulnerability DB: v%d (updated: %s)",
				version.VulnerabilityDB.Version,
				version.VulnerabilityDB.UpdatedAt.Format(time.RFC3339)))
		}
		if version.JavaDB != nil {
			task.AddLog(fmt.Sprintf("Java DB: v%d (updated: %s)",
				version.JavaDB.Version,
				version.JavaDB.UpdatedAt.Format(time.RFC3339)))
		}
		s.repo.Update(task)
	}

	// Build trivy command
	args := s.buildTrivyArgs(task)
	task.AddLog(fmt.Sprintf("Executing: trivy %s", strings.Join(s.maskCredentials(args), " ")))

	// Execute command with streaming logs
	stdout, stderr, cmdErr := s.executor.ExecuteCommand(ctx, "trivy", args, func(line string) {
		task.AddLog(line)
	})

	// Handle command result
	if cmdErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			s.failTask(task, "Scan timeout exceeded")
		} else {
			s.failTask(task, fmt.Sprintf("Scan failed: %v\n%s", cmdErr, stderr))
		}
		return
	}

	// Parse scan results
	result, err := s.parseScanResult(stdout, task.ScanConfig.Format)
	if err != nil {
		s.failTask(task, fmt.Sprintf("Failed to parse scan result: %v", err))
		return
	}

	// Save report to file
	reportPath, err := s.saveReport(task.ID, task.ScanConfig.Format, stdout)
	if err != nil {
		s.logger.Error("Failed to save report for task %s: %v", task.ID, err)
		// Don't fail the task, just log the error
	} else {
		task.AddLog(fmt.Sprintf("Report saved to: %s", reportPath))
	}

	// Update task with success
	endTime := time.Now()
	task.Status = models.ScanStatusCompleted
	task.Message = "Scan completed successfully"
	task.EndTime = &endTime
	task.Result = result
	task.Output = stdout
	task.AddLog(fmt.Sprintf("Scan completed at %s", endTime.Format(time.RFC3339)))
	task.CloseAllLogListeners()
	s.repo.Update(task)

	s.logger.Info("Scan completed for task %s", task.ID)
}

// buildTrivyArgs builds the trivy command arguments from task configuration.
func (s *scanServiceImpl) buildTrivyArgs(task *models.ScanTask) []string {
	args := []string{"image"}

	// Trivy server connection
	if s.config.ServerURL != "" {
		args = append(args, "--server", s.config.ServerURL)
	}

	// Skip database updates (managed by Trivy Server in client-server mode)
	// Note: Only skip main DB update. Do NOT skip Java DB update to allow server
	// to automatically initialize/update Java DB when needed (especially on first run)
	args = append(args, "--skip-db-update")
	// args = append(args, "--skip-java-db-update")

	// Force remote image source (pull from registry instead of local Docker/Containerd/Podman)
	args = append(args, "--image-src", "remote")

	// Timeout for Trivy internal operations (registry access, scanning, etc.)
	args = append(args, "--timeout", "10m")

	// Authentication
	if task.ScanConfig.Username != "" {
		args = append(args, "--username", task.ScanConfig.Username)
	}
	if task.ScanConfig.Password != "" {
		args = append(args, "--password", task.ScanConfig.Password)
	}

	// TLS verification
	if !task.ScanConfig.TLSVerify {
		args = append(args, "--insecure")
	}

	// Severity filter
	if len(task.ScanConfig.Severity) > 0 {
		args = append(args, "--severity", strings.Join(task.ScanConfig.Severity, ","))
	}

	// Ignore unfixed
	if task.ScanConfig.IgnoreUnfixed {
		args = append(args, "--ignore-unfixed")
	}

	// Scanners
	if len(task.ScanConfig.Scanners) > 0 {
		args = append(args, "--scanners", strings.Join(task.ScanConfig.Scanners, ","))
	}

	// Detection priority
	if task.ScanConfig.DetectionPriority != "" {
		args = append(args, "--detection-priority", task.ScanConfig.DetectionPriority)
	}

	// Package types
	if len(task.ScanConfig.PkgTypes) > 0 {
		args = append(args, "--pkg-types", strings.Join(task.ScanConfig.PkgTypes, ","))
	}

	// Output format
	if task.ScanConfig.Format != "" {
		args = append(args, "--format", task.ScanConfig.Format)
	}

	// Image to scan
	args = append(args, task.Image)

	return args
}

// maskCredentials masks sensitive information in command arguments.
func (s *scanServiceImpl) maskCredentials(args []string) []string {
	masked := make([]string, len(args))
	copy(masked, args)

	for i := 0; i < len(masked)-1; i++ {
		if masked[i] == "--password" || masked[i] == "--username" {
			masked[i+1] = "***"
		}
	}

	return masked
}

// parseScanResult parses trivy output and extracts vulnerability summary.
func (s *scanServiceImpl) parseScanResult(output, format string) (*models.ScanResult, error) {
	result := &models.ScanResult{
		Format: format,
		Data:   output,
	}

	// Parse vulnerability summary only for JSON format
	if format == "json" {
		summary, err := s.parseVulnerabilitySummary(output)
		if err != nil {
			s.logger.Error("Failed to parse vulnerability summary: %v", err)
			// Don't fail, just skip summary
		} else {
			result.Summary = summary
		}
	}

	return result, nil
}

// parseVulnerabilitySummary parses trivy JSON output to extract vulnerability statistics.
func (s *scanServiceImpl) parseVulnerabilitySummary(jsonOutput string) (*models.VulnerabilitySummary, error) {
	var trivyOutput struct {
		Results []struct {
			Vulnerabilities []struct {
				Severity string `json:"Severity"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}

	if err := json.Unmarshal([]byte(jsonOutput), &trivyOutput); err != nil {
		return nil, err
	}

	summary := &models.VulnerabilitySummary{}

	for _, result := range trivyOutput.Results {
		for _, vuln := range result.Vulnerabilities {
			summary.Total++
			switch vuln.Severity {
			case "CRITICAL":
				summary.Critical++
			case "HIGH":
				summary.High++
			case "MEDIUM":
				summary.Medium++
			case "LOW":
				summary.Low++
			default:
				summary.Unknown++
			}
		}
	}

	return summary, nil
}

// saveReport saves the scan report to disk with user isolation.
func (s *scanServiceImpl) saveReport(taskID, format, data string) (string, error) {
	// Get task to retrieve userID
	task, err := s.repo.GetByID(taskID)
	if err != nil {
		return "", fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return "", fmt.Errorf("task not found")
	}

	// Create user-specific reports directory: reports/users/{userID}/
	reportsDir := filepath.Join(s.storageDir, "reports", "users", task.UserID)
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create reports directory: %w", err)
	}

	// Determine file extension
	ext := format
	if ext == "json" {
		ext = "json"
	} else if ext == "table" {
		ext = "txt"
	}

	// Save report
	filename := fmt.Sprintf("%s.%s", taskID, ext)
	filepath := filepath.Join(reportsDir, filename)

	if err := os.WriteFile(filepath, []byte(data), 0644); err != nil {
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	return filepath, nil
}

// failTask marks a task as failed with error message.
func (s *scanServiceImpl) failTask(task *models.ScanTask, errorMsg string) {
	endTime := time.Now()
	task.Status = models.ScanStatusFailed
	task.Message = "Scan failed"
	task.EndTime = &endTime
	task.ErrorOutput = errorMsg
	task.AddLog(fmt.Sprintf("ERROR: %s", errorMsg))
	task.AddLog(fmt.Sprintf("Scan failed at %s", endTime.Format(time.RFC3339)))
	task.CloseAllLogListeners()
	s.repo.Update(task)

	s.logger.Error("Scan failed for task %s: %s", task.ID, errorMsg)
}

// GetTask retrieves a scan task by ID.
func (s *scanServiceImpl) GetTask(taskID string) (*models.ScanTask, error) {
	task, err := s.repo.GetByID(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	return task, nil
}

// ListTasks retrieves scan tasks with pagination and filtering.
func (s *scanServiceImpl) ListTasks(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error) {
	// Validate and set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}
	if req.SortBy == "" {
		req.SortBy = "startTime"
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	// Get tasks from repository
	tasks, total, err := s.repo.List(userID, req)
	if err != nil {
		return nil, err
	}

	// Convert to summaries
	summaries := make([]*models.TaskSummary, len(tasks))
	for i, task := range tasks {
		summaries[i] = task.ToSummary()
	}

	// Update queue positions for queued tasks
	queuedTasks, _ := s.repo.GetQueuedTasks(userID)
	for i, qt := range queuedTasks {
		for _, summary := range summaries {
			if summary.ID == qt.ID {
				summary.QueuePosition = i + 1
				break
			}
		}
	}

	return &models.TaskListResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Tasks:    summaries,
	}, nil
}

// GetQueueStatus returns the current queue status for a user.
func (s *scanServiceImpl) GetQueueStatus(userID string) (*models.QueueStatusResponse, error) {
	queuedTasks, err := s.repo.GetQueuedTasks(userID)
	if err != nil {
		return nil, err
	}

	// Calculate average wait time (simplified)
	// In production, this should be based on historical data
	avgWaitTime := float64(len(queuedTasks)) * 30.0 // Assume 30 seconds per task

	return &models.QueueStatusResponse{
		QueueLength:     len(queuedTasks),
		AverageWaitTime: avgWaitTime,
	}, nil
}

// ListDockerImages lists Docker images from the host machine.
func (s *scanServiceImpl) ListDockerImages() ([]models.DockerImage, error) {
	s.logger.Info("Listing Docker images from host")

	// Execute docker images command with JSON format
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{"images", "--format", "{{json .}}"}
	stdout, stderr, err := s.executor.ExecuteCommand(ctx, "docker", args, nil)

	if err != nil {
		return nil, fmt.Errorf("docker command failed: %w (stderr: %s)", err, stderr)
	}

	// Parse output (each line is a JSON object)
	var images []models.DockerImage
	lines := strings.Split(strings.TrimSpace(stdout), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var dockerImg struct {
			Repository string `json:"Repository"`
			Tag        string `json:"Tag"`
			ID         string `json:"ID"`
			CreatedAt  string `json:"CreatedAt"`
			Size       string `json:"Size"`
		}

		if err := json.Unmarshal([]byte(line), &dockerImg); err != nil {
			s.logger.Error("Failed to parse docker image line: %v (line: %s)", err, line)
			continue
		}

		// Skip <none> images
		if dockerImg.Repository == "<none>" || dockerImg.Tag == "<none>" {
			continue
		}

		fullName := dockerImg.Repository
		if dockerImg.Tag != "" {
			fullName = fmt.Sprintf("%s:%s", dockerImg.Repository, dockerImg.Tag)
		}

		images = append(images, models.DockerImage{
			Repository: dockerImg.Repository,
			Tag:        dockerImg.Tag,
			ImageID:    dockerImg.ID,
			Created:    dockerImg.CreatedAt,
			Size:       dockerImg.Size,
			FullName:   fullName,
		})
	}

	s.logger.Info("Found %d Docker images", len(images))
	return images, nil
}

// cleanupOldReports deletes scan reports older than retention period.
func (s *scanServiceImpl) cleanupOldReports() {
	s.logger.Info("Starting cleanup of old scan reports (retention: %d days)", s.config.ScanRetentionDays)

	// Calculate cutoff time
	cutoffTime := time.Now().AddDate(0, 0, -s.config.ScanRetentionDays)
	s.logger.Info("Deleting reports older than %s", cutoffTime.Format(time.RFC3339))

	// Get all old tasks from repository
	oldTasks, err := s.repo.GetAllOldTasks(cutoffTime)
	if err != nil {
		s.logger.Error("Failed to get old tasks for cleanup: %v", err)
		return
	}

	if len(oldTasks) == 0 {
		s.logger.Info("No old reports to clean up")
		return
	}

	s.logger.Info("Found %d old reports to delete", len(oldTasks))

	// Delete tasks and their reports
	deletedCount := 0
	deletedSize := int64(0)

	for _, task := range oldTasks {
		// Delete report file
		reportSize, err := s.deleteReport(task.ID)
		if err != nil {
			s.logger.Error("Failed to delete report for task %s: %v", task.ID, err)
			// Continue with next task
		} else {
			deletedSize += reportSize
		}

		// Delete task from repository
		if err := s.repo.Delete(task.ID); err != nil {
			s.logger.Error("Failed to delete task %s from repository: %v", task.ID, err)
			continue
		}

		deletedCount++
	}

	// Log cleanup statistics
	s.logger.Info("Cleanup completed: deleted %d reports, freed %.2f MB",
		deletedCount, float64(deletedSize)/(1024*1024))
}

// deleteReport deletes the report file for a task with user isolation.
// Returns the size of deleted file in bytes.
func (s *scanServiceImpl) deleteReport(taskID string) (int64, error) {
	// Get task to retrieve userID
	task, err := s.repo.GetByID(taskID)
	if err != nil {
		return 0, fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return 0, fmt.Errorf("task not found")
	}

	// User-specific reports directory: reports/users/{userID}/
	reportsDir := filepath.Join(s.storageDir, "reports", "users", task.UserID)

	// Try all possible extensions
	extensions := []string{"json", "txt"}
	var totalSize int64

	for _, ext := range extensions {
		filename := fmt.Sprintf("%s.%s", taskID, ext)
		filepath := filepath.Join(reportsDir, filename)

		info, err := os.Stat(filepath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // File doesn't exist, try next extension
			}
			return 0, err
		}

		// Delete file
		if err := os.Remove(filepath); err != nil {
			return 0, err
		}

		totalSize += info.Size()
	}

	return totalSize, nil
}

// ListDockerContainers lists running Docker containers from the host machine.
func (s *scanServiceImpl) ListDockerContainers() ([]models.DockerContainer, error) {
	s.logger.Info("Listing Docker containers from host")

	// Execute docker ps command with custom format (only get fields we need)
	// This is MUCH faster than {{json .}} which includes huge Labels and Mounts fields
	// Performance: custom format ~50ms vs {{json .}} ~12s for 60 containers
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Custom format: only get the fields we actually need (pipe-separated)
	format := "{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.Ports}}|{{.CreatedAt}}"
	args := []string{"ps", "--format", format}
	stdout, stderr, err := s.executor.ExecuteCommand(ctx, "docker", args, nil)

	if err != nil {
		return nil, fmt.Errorf("docker command failed: %w (stderr: %s)", err, stderr)
	}

	// Parse output (each line is pipe-separated values)
	var containers []models.DockerContainer
	lines := strings.Split(strings.TrimSpace(stdout), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Split by pipe separator
		parts := strings.SplitN(line, "|", 6)
		if len(parts) != 6 {
			s.logger.Error("Failed to parse docker container line: unexpected format (expected 6 parts, got %d) (line: %s)", len(parts), line)
			continue
		}

		containers = append(containers, models.DockerContainer{
			ContainerID:   parts[0],
			ContainerName: parts[1],
			Image:         parts[2],
			Status:        parts[3],
			Ports:         parts[4],
			Created:       parts[5],
		})
	}

	s.logger.Info("Found %d running Docker containers", len(containers))
	return containers, nil
}

// DeleteTask deletes a scan task and its report files.
func (s *scanServiceImpl) DeleteTask(taskID string) error {
	// Delete report files
	if _, err := s.deleteReport(taskID); err != nil {
		s.logger.Error("Failed to delete report for task %s: %v", taskID, err)
		// Continue with task deletion even if report deletion fails
	}

	// Delete task from repository
	if err := s.repo.Delete(taskID); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	s.logger.Info("Deleted task %s and its reports", taskID)
	return nil
}

// markInterruptedTasksAsFailed marks all running tasks as failed on service startup.
// This handles the case where the server was restarted while tasks were in progress.
func (s *scanServiceImpl) markInterruptedTasksAsFailed() {
	runningTasks, err := s.repo.GetAllRunningTasks()
	if err != nil {
		s.logger.Error("Failed to get running tasks on startup: %v", err)
		return
	}

	if len(runningTasks) == 0 {
		s.logger.Info("No interrupted tasks to recover")
		return
	}

	s.logger.Info("Marking %d interrupted tasks as failed", len(runningTasks))

	for _, task := range runningTasks {
		endTime := time.Now()
		task.Status = models.ScanStatusFailed
		task.Message = "Scan interrupted by server restart"
		task.EndTime = &endTime
		task.ErrorOutput = "Server was restarted while this scan was in progress"
		task.AddLog("ERROR: Scan interrupted by server restart")
		task.AddLog(fmt.Sprintf("Task marked as failed at %s", endTime.Format(time.RFC3339)))
		task.CloseAllLogListeners()

		if err := s.repo.Update(task); err != nil {
			s.logger.Error("Failed to update interrupted task %s: %v", task.ID, err)
		} else {
			s.logger.Info("Marked interrupted task %s as failed", task.ID)
		}
	}
}

// DeleteAllTasks deletes all scan tasks and their report files for a user.
func (s *scanServiceImpl) DeleteAllTasks(userID string) error {
	s.logger.Info("Deleting all tasks for user %s", userID)

	// Get all tasks for the user
	tasks, _, err := s.repo.List(userID, &models.TaskListRequest{
		Page:     1,
		PageSize: 10000, // Large number to get all tasks
	})
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	if len(tasks) == 0 {
		s.logger.Info("No tasks to delete for user %s", userID)
		return nil
	}

	s.logger.Info("Found %d tasks to delete for user %s", len(tasks), userID)

	deletedCount := 0
	var deletedSize int64

	for _, task := range tasks {
		// Delete report files
		size, err := s.deleteReport(task.ID)
		if err != nil {
			s.logger.Error("Failed to delete report for task %s: %v", task.ID, err)
		} else {
			deletedSize += size
		}

		// Delete task from repository
		if err := s.repo.Delete(task.ID); err != nil {
			s.logger.Error("Failed to delete task %s: %v", task.ID, err)
			continue
		}

		deletedCount++
	}

	s.logger.Info("Deleted %d tasks for user %s, freed %.2f MB",
		deletedCount, userID, float64(deletedSize)/(1024*1024))

	return nil
}

// GetTrivyVersion retrieves version information from Trivy Server.
func (s *scanServiceImpl) GetTrivyVersion(ctx context.Context) (*models.TrivyVersion, error) {
	if s.config.ServerURL == "" {
		return nil, fmt.Errorf("trivy server URL not configured")
	}

	// Build version endpoint URL
	versionURL := strings.TrimRight(s.config.ServerURL, "/") + "/version"
	s.logger.Info("Fetching Trivy Server version from: %s", versionURL)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("version endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var version models.TrivyVersion
	if err := json.Unmarshal(body, &version); err != nil {
		return nil, fmt.Errorf("failed to parse version JSON: %w", err)
	}

	s.logger.Info("Retrieved Trivy Server version: %s", version.Version)
	return &version, nil
}
