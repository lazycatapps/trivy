// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package repository provides data access layer for scan tasks.
package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lazycatapps/trivy/backend/internal/models"
)

// ScanRepository defines the interface for scan task storage operations.
type ScanRepository interface {
	// Create adds a new scan task to the repository.
	Create(task *models.ScanTask) error

	// GetByID retrieves a scan task by its unique identifier.
	// Returns nil if the task does not exist.
	GetByID(id string) (*models.ScanTask, error)

	// List retrieves scan tasks with pagination and filtering.
	// Supports filtering by user ID, status, and sorting.
	List(userID string, filter *models.TaskListRequest) ([]*models.ScanTask, int, error)

	// Update updates an existing scan task.
	Update(task *models.ScanTask) error

	// Delete removes a scan task from the repository.
	Delete(id string) error

	// GetQueuedTasks retrieves all tasks in queued status for a specific user.
	// Returns tasks ordered by creation time (FIFO).
	GetQueuedTasks(userID string) ([]*models.ScanTask, error)

	// GetRunningTask retrieves the currently running task for a user.
	// Returns nil if no task is running.
	GetRunningTask(userID string) (*models.ScanTask, error)

	// GetAllRunningTasks retrieves all tasks that are currently in running status.
	// Used during startup to mark interrupted tasks as failed.
	GetAllRunningTasks() ([]*models.ScanTask, error)

	// GetAllOldTasks retrieves all tasks older than the specified cutoff time.
	// Returns tasks across all users (for global cleanup).
	GetAllOldTasks(cutoffTime time.Time) ([]*models.ScanTask, error)
}

// InMemoryScanRepository implements ScanRepository with in-memory storage.
// Thread-safe for concurrent access.
type InMemoryScanRepository struct {
	tasks map[string]*models.ScanTask // Map of task ID to ScanTask
	mu    sync.RWMutex                // Mutex for thread-safe operations
}

// NewInMemoryScanRepository creates a new in-memory scan repository.
func NewInMemoryScanRepository() *InMemoryScanRepository {
	return &InMemoryScanRepository{
		tasks: make(map[string]*models.ScanTask),
	}
}

// Create adds a new scan task to the repository.
func (r *InMemoryScanRepository) Create(task *models.ScanTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	r.tasks[task.ID] = task
	return nil
}

// GetByID retrieves a scan task by its unique identifier.
func (r *InMemoryScanRepository) GetByID(id string) (*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, exists := r.tasks[id]
	if !exists {
		return nil, nil // Task not found
	}

	return task, nil
}

// List retrieves scan tasks with pagination and filtering.
func (r *InMemoryScanRepository) List(userID string, filter *models.TaskListRequest) ([]*models.ScanTask, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Filter tasks by user ID and status
	var filtered []*models.ScanTask
	for _, task := range r.tasks {
		// Filter by user ID
		if task.UserID != userID {
			continue
		}

		// Filter by status (optional)
		if filter.Status != "" && string(task.Status) != filter.Status {
			continue
		}

		filtered = append(filtered, task)
	}

	total := len(filtered)

	// Sort tasks
	sortTasks(filtered, filter.SortBy, filter.SortOrder)

	// Apply pagination
	start := (filter.Page - 1) * filter.PageSize
	end := start + filter.PageSize

	if start >= total {
		return []*models.ScanTask{}, total, nil
	}

	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

// Update updates an existing scan task.
func (r *InMemoryScanRepository) Update(task *models.ScanTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[task.ID]; !exists {
		return fmt.Errorf("task with ID %s does not exist", task.ID)
	}

	r.tasks[task.ID] = task
	return nil
}

// Delete removes a scan task from the repository.
func (r *InMemoryScanRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[id]; !exists {
		return fmt.Errorf("task with ID %s does not exist", id)
	}

	delete(r.tasks, id)
	return nil
}

// GetQueuedTasks retrieves all tasks in queued status for a specific user.
func (r *InMemoryScanRepository) GetQueuedTasks(userID string) ([]*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var queued []*models.ScanTask
	for _, task := range r.tasks {
		if task.UserID == userID && task.Status == models.ScanStatusQueued {
			queued = append(queued, task)
		}
	}

	// Sort by start time (FIFO)
	sort.Slice(queued, func(i, j int) bool {
		return queued[i].StartTime.Before(queued[j].StartTime)
	})

	return queued, nil
}

// GetRunningTask retrieves the currently running task for a user.
func (r *InMemoryScanRepository) GetRunningTask(userID string) (*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, task := range r.tasks {
		if task.UserID == userID && task.Status == models.ScanStatusRunning {
			return task, nil
		}
	}

	return nil, nil // No running task
}

// GetAllRunningTasks retrieves all tasks that are currently in running status.
func (r *InMemoryScanRepository) GetAllRunningTasks() ([]*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var runningTasks []*models.ScanTask
	for _, task := range r.tasks {
		if task.Status == models.ScanStatusRunning {
			runningTasks = append(runningTasks, task)
		}
	}

	return runningTasks, nil
}

// GetAllOldTasks retrieves all tasks older than the specified cutoff time.
func (r *InMemoryScanRepository) GetAllOldTasks(cutoffTime time.Time) ([]*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var oldTasks []*models.ScanTask
	for _, task := range r.tasks {
		// Only consider completed or failed tasks
		if task.Status != models.ScanStatusCompleted && task.Status != models.ScanStatusFailed {
			continue
		}

		// Check if task is older than cutoff time
		// Use EndTime if available, otherwise use StartTime
		taskTime := task.StartTime
		if task.EndTime != nil {
			taskTime = *task.EndTime
		}

		if taskTime.Before(cutoffTime) {
			oldTasks = append(oldTasks, task)
		}
	}

	return oldTasks, nil
}

// sortTasks sorts tasks by the specified field and order.
func sortTasks(tasks []*models.ScanTask, sortBy, sortOrder string) {
	sort.Slice(tasks, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "startTime":
			less = tasks[i].StartTime.Before(tasks[j].StartTime)
		case "endTime":
			if tasks[i].EndTime == nil {
				less = false
			} else if tasks[j].EndTime == nil {
				less = true
			} else {
				less = tasks[i].EndTime.Before(*tasks[j].EndTime)
			}
		default:
			// Default to startTime
			less = tasks[i].StartTime.Before(tasks[j].StartTime)
		}

		// Reverse order for descending sort
		if sortOrder == "desc" {
			less = !less
		}

		return less
	})
}

// FileBasedScanRepository implements ScanRepository with file system persistence.
// Tasks are stored in a directory structure organized by user ID.
// Thread-safe for concurrent access.
type FileBasedScanRepository struct {
	baseDir string                      // Base directory for storing scan data
	cache   map[string]*models.ScanTask // In-memory cache for fast access
	mu      sync.RWMutex                // Mutex for thread-safe operations
}

// NewFileBasedScanRepository creates a new file-based scan repository.
// Directory structure:
//   - {baseDir}/scans/users/{userID}/{taskID}/metadata.json (OIDC enabled)
//   - {baseDir}/scans/users/{userID}/{taskID}/result.json
//   - {baseDir}/scans/shared/{taskID}/metadata.json (OIDC disabled)
//   - {baseDir}/scans/shared/{taskID}/result.json
func NewFileBasedScanRepository(baseDir string) (*FileBasedScanRepository, error) {
	// Create base directory if not exists
	scansDir := filepath.Join(baseDir, "scans")
	if err := os.MkdirAll(scansDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create scans directory: %w", err)
	}

	repo := &FileBasedScanRepository{
		baseDir: baseDir,
		cache:   make(map[string]*models.ScanTask),
	}

	// Load all existing tasks into cache
	if err := repo.loadAllTasks(); err != nil {
		return nil, fmt.Errorf("failed to load existing tasks: %w", err)
	}

	return repo, nil
}

// sanitizeUserIdentifier sanitizes user identifier for safe file system usage.
// This prevents path traversal and ensures consistent directory naming.
// Reference: image-sync project's config_service.go
func sanitizeUserIdentifier(identifier string) string {
	if identifier == "" {
		return ""
	}

	const maxLength = 128
	const allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-@"

	var builder strings.Builder
	for _, r := range identifier {
		if strings.ContainsRune(allowed, r) {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('-')
		}
	}

	safe := builder.String()
	safe = strings.ReplaceAll(safe, "..", "-")
	safe = strings.Trim(safe, "-_.")

	if safe == "" {
		// If all characters are invalid, use hash of original identifier
		sum := sha256.Sum256([]byte(identifier))
		return hex.EncodeToString(sum[:8])
	}

	if len(safe) > maxLength {
		safe = safe[:maxLength]
	}

	return safe
}

// getTaskDir returns the directory path for a specific task.
func (r *FileBasedScanRepository) getTaskDir(userID, taskID string) string {
	if userID != "" {
		safeUserID := sanitizeUserIdentifier(userID)
		return filepath.Join(r.baseDir, "scans/users", safeUserID, taskID)
	}
	return filepath.Join(r.baseDir, "scans/shared", taskID)
}

// getUserDir returns the directory path for a specific user's scans.
func (r *FileBasedScanRepository) getUserDir(userID string) string {
	if userID != "" {
		safeUserID := sanitizeUserIdentifier(userID)
		return filepath.Join(r.baseDir, "scans/users", safeUserID)
	}
	return filepath.Join(r.baseDir, "scans/shared")
}

// loadAllTasks loads all existing tasks from file system into cache.
func (r *FileBasedScanRepository) loadAllTasks() error {
	scansDir := filepath.Join(r.baseDir, "scans")

	// Load from both users/ and shared/ directories
	dirs := []string{
		filepath.Join(scansDir, "users"),
		filepath.Join(scansDir, "shared"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // Directory doesn't exist yet, skip
		}

		// Walk through all subdirectories
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Only process metadata.json files
			if !info.IsDir() && info.Name() == "metadata.json" {
				task, err := r.loadTaskFromFile(path)
				if err != nil {
					// Log error but continue loading other tasks
					return nil
				}
				r.cache[task.ID] = task
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// loadTaskFromFile loads a single task from metadata.json file.
func (r *FileBasedScanRepository) loadTaskFromFile(metadataPath string) (*models.ScanTask, error) {
	data, err := ioutil.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var task models.ScanTask
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Initialize transient fields (not persisted)
	task.LogLines = []string{}
	task.LogListeners = []chan string{}

	// Load result.json if exists
	taskDir := filepath.Dir(metadataPath)
	resultPath := filepath.Join(taskDir, "result.json")
	if _, err := os.Stat(resultPath); err == nil {
		resultData, err := ioutil.ReadFile(resultPath)
		if err == nil {
			var result models.ScanResult
			if err := json.Unmarshal(resultData, &result); err == nil {
				task.Result = &result
			}
		}
	}

	return &task, nil
}

// saveTaskToFile saves a task's metadata to file system.
func (r *FileBasedScanRepository) saveTaskToFile(task *models.ScanTask) error {
	taskDir := r.getTaskDir(task.UserID, task.ID)

	// Create task directory
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("failed to create task directory: %w", err)
	}

	// Create a copy without transient fields (avoid copying mutex)
	taskCopy := models.ScanTask{
		ID:            task.ID,
		UserID:        task.UserID,
		Image:         task.Image,
		Status:        task.Status,
		Message:       task.Message,
		StartTime:     task.StartTime,
		EndTime:       task.EndTime,
		ScanConfig:    task.ScanConfig,
		QueuePosition: task.QueuePosition,
		Result:        task.Result,
		Output:        task.Output,
		ErrorOutput:   task.ErrorOutput,
		// Explicitly omit: LogLines, LogListeners, logMu
	}

	// Save metadata.json
	metadataPath := filepath.Join(taskDir, "metadata.json")
	data, err := json.MarshalIndent(&taskCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := ioutil.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	// Save result.json if result exists
	if task.Result != nil {
		resultPath := filepath.Join(taskDir, "result.json")
		resultData, err := json.MarshalIndent(task.Result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}

		if err := ioutil.WriteFile(resultPath, resultData, 0644); err != nil {
			return fmt.Errorf("failed to write result file: %w", err)
		}
	}

	return nil
}

// Create adds a new scan task to the repository.
func (r *FileBasedScanRepository) Create(task *models.ScanTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.cache[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	// Save to file system
	if err := r.saveTaskToFile(task); err != nil {
		return err
	}

	// Add to cache
	r.cache[task.ID] = task
	return nil
}

// GetByID retrieves a scan task by its unique identifier.
func (r *FileBasedScanRepository) GetByID(id string) (*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, exists := r.cache[id]
	if !exists {
		return nil, nil // Task not found
	}

	return task, nil
}

// List retrieves scan tasks with pagination and filtering.
func (r *FileBasedScanRepository) List(userID string, filter *models.TaskListRequest) ([]*models.ScanTask, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Filter tasks by user ID and status
	var filtered []*models.ScanTask
	for _, task := range r.cache {
		// Filter by user ID
		if task.UserID != userID {
			continue
		}

		// Filter by status (optional)
		if filter.Status != "" && string(task.Status) != filter.Status {
			continue
		}

		filtered = append(filtered, task)
	}

	total := len(filtered)

	// Sort tasks
	sortTasks(filtered, filter.SortBy, filter.SortOrder)

	// Apply pagination
	start := (filter.Page - 1) * filter.PageSize
	end := start + filter.PageSize

	if start >= total {
		return []*models.ScanTask{}, total, nil
	}

	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

// Update updates an existing scan task.
func (r *FileBasedScanRepository) Update(task *models.ScanTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.cache[task.ID]; !exists {
		return fmt.Errorf("task with ID %s does not exist", task.ID)
	}

	// Save to file system
	if err := r.saveTaskToFile(task); err != nil {
		return err
	}

	// Update cache
	r.cache[task.ID] = task
	return nil
}

// Delete removes a scan task from the repository.
func (r *FileBasedScanRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, exists := r.cache[id]
	if !exists {
		return fmt.Errorf("task with ID %s does not exist", id)
	}

	// Remove from file system
	taskDir := r.getTaskDir(task.UserID, task.ID)
	if err := os.RemoveAll(taskDir); err != nil {
		return fmt.Errorf("failed to delete task directory: %w", err)
	}

	// Remove from cache
	delete(r.cache, id)
	return nil
}

// GetQueuedTasks retrieves all tasks in queued status for a specific user.
func (r *FileBasedScanRepository) GetQueuedTasks(userID string) ([]*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var queued []*models.ScanTask
	for _, task := range r.cache {
		if task.UserID == userID && task.Status == models.ScanStatusQueued {
			queued = append(queued, task)
		}
	}

	// Sort by start time (FIFO)
	sort.Slice(queued, func(i, j int) bool {
		return queued[i].StartTime.Before(queued[j].StartTime)
	})

	return queued, nil
}

// GetAllRunningTasks retrieves all tasks that are currently in running status.
func (r *FileBasedScanRepository) GetAllRunningTasks() ([]*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var runningTasks []*models.ScanTask
	for _, task := range r.cache {
		if task.Status == models.ScanStatusRunning {
			runningTasks = append(runningTasks, task)
		}
	}

	return runningTasks, nil
}

// GetRunningTask retrieves the currently running task for a user.
func (r *FileBasedScanRepository) GetRunningTask(userID string) (*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, task := range r.cache {
		if task.UserID == userID && task.Status == models.ScanStatusRunning {
			return task, nil
		}
	}

	return nil, nil // No running task
}

// GetAllOldTasks retrieves all tasks older than the specified cutoff time.
func (r *FileBasedScanRepository) GetAllOldTasks(cutoffTime time.Time) ([]*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var oldTasks []*models.ScanTask
	for _, task := range r.cache {
		// Only consider completed or failed tasks
		if task.Status != models.ScanStatusCompleted && task.Status != models.ScanStatusFailed {
			continue
		}

		// Check if task is older than cutoff time
		// Use EndTime if available, otherwise use StartTime
		taskTime := task.StartTime
		if task.EndTime != nil {
			taskTime = *task.EndTime
		}

		if taskTime.Before(cutoffTime) {
			oldTasks = append(oldTasks, task)
		}
	}

	return oldTasks, nil
}

// GetUserTaskCount returns the total number of tasks for a specific user.
func (r *FileBasedScanRepository) GetUserTaskCount(userID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, task := range r.cache {
		if task.UserID == userID {
			count++
		}
	}

	return count, nil
}

// GetUserStorageSize returns the total storage size (in bytes) used by a user's scans.
func (r *FileBasedScanRepository) GetUserStorageSize(userID string) (int64, error) {
	userDir := r.getUserDir(userID)

	var totalSize int64
	err := filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if os.IsNotExist(err) {
		return 0, nil // User directory doesn't exist yet
	}

	return totalSize, err
}

// getUserIDFromPath extracts user ID from a task directory path.
func getUserIDFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part == "users" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "" // Shared directory (no user ID)
}
