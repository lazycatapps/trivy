// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package repository provides data access layer for scan tasks.
package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/lazycatapps/trivy/backend/internal/models"
)

// FileScanRepository implements ScanRepository with file-based storage.
// Thread-safe for concurrent access.
// Stores scan tasks as JSON files in the configured directory.
type FileScanRepository struct {
	baseDir string                      // Base directory for scan task storage
	tasks   map[string]*models.ScanTask // In-memory cache of tasks
	mu      sync.RWMutex                // Mutex for thread-safe operations
}

// NewFileScanRepository creates a new file-based scan repository.
// Tasks are stored in: <baseDir>/scans/<userID>/scan_<taskID>.json
func NewFileScanRepository(baseDir string) (*FileScanRepository, error) {
	repo := &FileScanRepository{
		baseDir: baseDir,
		tasks:   make(map[string]*models.ScanTask),
	}

	// Load existing tasks from disk
	if err := repo.loadTasks(); err != nil {
		return nil, fmt.Errorf("failed to load existing tasks: %w", err)
	}

	return repo, nil
}

// loadTasks loads all scan tasks from disk into memory cache.
func (r *FileScanRepository) loadTasks() error {
	scansDir := filepath.Join(r.baseDir, "scans")

	// If scans directory doesn't exist, nothing to load
	if _, err := os.Stat(scansDir); os.IsNotExist(err) {
		return nil
	}

	// Walk through all user directories
	userDirs, err := os.ReadDir(scansDir)
	if err != nil {
		return fmt.Errorf("failed to read scans directory: %w", err)
	}

	for _, userDir := range userDirs {
		if !userDir.IsDir() {
			continue
		}

		userScansDir := filepath.Join(scansDir, userDir.Name())
		files, err := os.ReadDir(userScansDir)
		if err != nil {
			continue // Skip if can't read user directory
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}

			filePath := filepath.Join(userScansDir, file.Name())
			task, err := r.loadTaskFromFile(filePath)
			if err != nil {
				// Log error but continue loading other tasks
				continue
			}

			r.tasks[task.ID] = task
		}
	}

	return nil
}

// loadTaskFromFile loads a single task from a JSON file.
func (r *FileScanRepository) loadTaskFromFile(filePath string) (*models.ScanTask, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file: %w", err)
	}

	var task models.ScanTask
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

// saveTaskToFile saves a task to a JSON file.
func (r *FileScanRepository) saveTaskToFile(task *models.ScanTask) error {
	// Create user-specific directory
	userScansDir := filepath.Join(r.baseDir, "scans", task.UserID)
	if err := os.MkdirAll(userScansDir, 0755); err != nil {
		return fmt.Errorf("failed to create scans directory: %w", err)
	}

	// Serialize task to JSON
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Write to file
	filePath := filepath.Join(userScansDir, fmt.Sprintf("scan_%s.json", task.ID))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	return nil
}

// deleteTaskFile deletes a task file from disk.
func (r *FileScanRepository) deleteTaskFile(task *models.ScanTask) error {
	filePath := filepath.Join(r.baseDir, "scans", task.UserID, fmt.Sprintf("scan_%s.json", task.ID))
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete task file: %w", err)
	}
	return nil
}

// Create adds a new scan task to the repository.
func (r *FileScanRepository) Create(task *models.ScanTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	// Save to disk
	if err := r.saveTaskToFile(task); err != nil {
		return err
	}

	// Update in-memory cache
	r.tasks[task.ID] = task
	return nil
}

// GetByID retrieves a scan task by its unique identifier.
func (r *FileScanRepository) GetByID(id string) (*models.ScanTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, exists := r.tasks[id]
	if !exists {
		return nil, nil // Task not found
	}

	return task, nil
}

// List retrieves scan tasks with pagination and filtering.
func (r *FileScanRepository) List(userID string, filter *models.TaskListRequest) ([]*models.ScanTask, int, error) {
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
func (r *FileScanRepository) Update(task *models.ScanTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[task.ID]; !exists {
		return fmt.Errorf("task with ID %s does not exist", task.ID)
	}

	// Save to disk
	if err := r.saveTaskToFile(task); err != nil {
		return err
	}

	// Update in-memory cache
	r.tasks[task.ID] = task
	return nil
}

// Delete removes a scan task from the repository.
func (r *FileScanRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, exists := r.tasks[id]
	if !exists {
		return fmt.Errorf("task with ID %s does not exist", id)
	}

	// Delete from disk
	if err := r.deleteTaskFile(task); err != nil {
		return err
	}

	// Remove from in-memory cache
	delete(r.tasks, id)
	return nil
}

// GetQueuedTasks retrieves all tasks in queued status for a specific user.
func (r *FileScanRepository) GetQueuedTasks(userID string) ([]*models.ScanTask, error) {
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
func (r *FileScanRepository) GetRunningTask(userID string) (*models.ScanTask, error) {
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
func (r *FileScanRepository) GetAllRunningTasks() ([]*models.ScanTask, error) {
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
