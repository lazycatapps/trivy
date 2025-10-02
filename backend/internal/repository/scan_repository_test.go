// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lazycatapps/trivy/backend/internal/models"
)

// TestCreate tests creating a scan task
func TestCreate(t *testing.T) {
	repo := NewInMemoryScanRepository()

	task := models.NewScanTask("task-1", "user-1", "alpine:latest", &models.ScanConfig{
		TLSVerify: true,
		Format:    "json",
	})

	err := repo.Create(task)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Verify task was created
	retrieved, err := repo.GetByID("task-1")
	if err != nil {
		t.Fatalf("Failed to retrieve task: %v", err)
	}

	if retrieved.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, retrieved.ID)
	}

	// Test creating duplicate task
	err = repo.Create(task)
	if err == nil {
		t.Error("Expected error when creating duplicate task")
	}
}

// TestGetByID tests retrieving a task by ID
func TestGetByID(t *testing.T) {
	repo := NewInMemoryScanRepository()

	// Create a task
	task := models.NewScanTask("task-1", "user-1", "alpine:latest", &models.ScanConfig{
		TLSVerify: true,
		Format:    "json",
	})
	repo.Create(task)

	tests := []struct {
		name      string
		taskID    string
		wantError bool
	}{
		{
			name:      "Get existing task",
			taskID:    "task-1",
			wantError: false,
		},
		{
			name:      "Get non-existent task",
			taskID:    "non-existent",
			wantError: false, // GetByID returns nil, nil for non-existent task
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrieved, err := repo.GetByID(tt.taskID)

			if tt.taskID == "non-existent" {
				// For non-existent task, expect nil task and no error
				if err != nil {
					t.Errorf("Expected no error for non-existent task, got %v", err)
				}
				if retrieved != nil {
					t.Error("Expected nil task for non-existent ID")
				}
				return
			}

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if retrieved != nil {
					t.Error("Expected nil task for error case")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if retrieved == nil {
				t.Fatal("Expected non-nil task")
			}

			if retrieved.ID != tt.taskID {
				t.Errorf("Expected task ID %s, got %s", tt.taskID, retrieved.ID)
			}
		})
	}
}

// TestUpdate tests updating a task
func TestUpdate(t *testing.T) {
	repo := NewInMemoryScanRepository()

	// Create a task
	task := models.NewScanTask("task-1", "user-1", "alpine:latest", &models.ScanConfig{
		TLSVerify: true,
		Format:    "json",
	})
	repo.Create(task)

	// Update task status
	task.Status = models.ScanStatusRunning
	task.Message = "Scanning in progress"

	err := repo.Update(task)
	if err != nil {
		t.Fatalf("Failed to update task: %v", err)
	}

	// Verify update
	retrieved, _ := repo.GetByID("task-1")
	if retrieved.Status != models.ScanStatusRunning {
		t.Errorf("Expected status %s, got %s", models.ScanStatusRunning, retrieved.Status)
	}
	if retrieved.Message != "Scanning in progress" {
		t.Errorf("Expected message 'Scanning in progress', got %s", retrieved.Message)
	}

	// Test updating non-existent task
	nonExistent := models.NewScanTask("non-existent", "user-1", "alpine:latest", &models.ScanConfig{})
	err = repo.Update(nonExistent)
	if err == nil {
		t.Error("Expected error when updating non-existent task")
	}
}

// TestDelete tests deleting a task
func TestDelete(t *testing.T) {
	repo := NewInMemoryScanRepository()

	// Create a task
	task := models.NewScanTask("task-1", "user-1", "alpine:latest", &models.ScanConfig{
		TLSVerify: true,
		Format:    "json",
	})
	repo.Create(task)

	// Delete task
	err := repo.Delete("task-1")
	if err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// Verify deletion
	retrieved, err := repo.GetByID("task-1")
	if err != nil {
		t.Errorf("Expected no error when getting deleted task, got %v", err)
	}
	if retrieved != nil {
		t.Error("Expected nil task after deletion")
	}

	// Test deleting non-existent task
	err = repo.Delete("non-existent")
	if err == nil {
		t.Error("Expected error when deleting non-existent task")
	}
}

// TestList tests listing tasks with pagination and filtering
func TestList(t *testing.T) {
	repo := NewInMemoryScanRepository()

	// Create multiple tasks for different users
	for i := 0; i < 10; i++ {
		userID := "user-1"
		if i >= 5 {
			userID = "user-2"
		}

		task := models.NewScanTask("task-"+string(rune('0'+i)), userID, "alpine:latest", &models.ScanConfig{
			TLSVerify: true,
			Format:    "json",
		})

		// Set different statuses
		if i%2 == 0 {
			task.Status = models.ScanStatusCompleted
			// Set endTime for completed tasks with different delays
			endTime := time.Now().Add(time.Duration(i*50) * time.Millisecond)
			task.EndTime = &endTime
		} else {
			task.Status = models.ScanStatusRunning
			// Running tasks have no endTime
		}

		// Add small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)

		repo.Create(task)
	}

	tests := []struct {
		name     string
		userID   string
		req      *models.TaskListRequest
		validate func(*testing.T, []*models.ScanTask, int)
	}{
		{
			name:   "List all tasks for user-1",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:     1,
				PageSize: 20,
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				if total != 5 {
					t.Errorf("Expected 5 total tasks for user-1, got %d", total)
				}
			},
		},
		{
			name:   "List with pagination",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:     1,
				PageSize: 2,
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				if len(tasks) > 2 {
					t.Errorf("Expected max 2 tasks per page, got %d", len(tasks))
				}
				if total != 5 {
					t.Errorf("Expected 5 total tasks, got %d", total)
				}
			},
		},
		{
			name:   "List with status filter",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:     1,
				PageSize: 20,
				Status:   string(models.ScanStatusCompleted),
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				for _, task := range tasks {
					if task.Status != models.ScanStatusCompleted {
						t.Errorf("Expected all tasks to have status completed, got %s", task.Status)
					}
				}
			},
		},
		{
			name:   "List with descending sort",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:      1,
				PageSize:  20,
				SortBy:    "startTime",
				SortOrder: "desc",
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				if len(tasks) < 2 {
					return
				}
				// Verify descending order
				for i := 0; i < len(tasks)-1; i++ {
					if tasks[i].StartTime.Before(tasks[i+1].StartTime) {
						t.Error("Tasks not sorted in descending order")
						break
					}
				}
			},
		},
		{
			name:   "List with ascending sort",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:      1,
				PageSize:  20,
				SortBy:    "startTime",
				SortOrder: "asc",
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				if len(tasks) < 2 {
					return
				}
				// Verify ascending order
				for i := 0; i < len(tasks)-1; i++ {
					if tasks[i].StartTime.After(tasks[i+1].StartTime) {
						t.Error("Tasks not sorted in ascending order")
						break
					}
				}
			},
		},
		{
			name:   "List with endTime descending sort",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:      1,
				PageSize:  20,
				SortBy:    "endTime",
				SortOrder: "desc",
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				if len(tasks) < 2 {
					return
				}
				// Verify descending order - nil endTime should be at the end
				for i := 0; i < len(tasks)-1; i++ {
					if tasks[i].EndTime != nil && tasks[i+1].EndTime != nil {
						if tasks[i].EndTime.Before(*tasks[i+1].EndTime) {
							t.Error("Tasks not sorted by endTime in descending order")
							break
						}
					}
				}
			},
		},
		{
			name:   "List with endTime ascending sort",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:      1,
				PageSize:  20,
				SortBy:    "endTime",
				SortOrder: "asc",
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				if len(tasks) < 2 {
					return
				}
				// Verify ascending order - nil endTime should be at the end
				for i := 0; i < len(tasks)-1; i++ {
					if tasks[i].EndTime != nil && tasks[i+1].EndTime != nil {
						if tasks[i].EndTime.After(*tasks[i+1].EndTime) {
							t.Error("Tasks not sorted by endTime in ascending order")
							break
						}
					}
				}
			},
		},
		{
			name:   "List with invalid sortBy uses default",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:      1,
				PageSize:  20,
				SortBy:    "invalid",
				SortOrder: "desc",
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				if len(tasks) < 2 {
					return
				}
				// Should fall back to startTime sorting
				for i := 0; i < len(tasks)-1; i++ {
					if tasks[i].StartTime.Before(tasks[i+1].StartTime) {
						t.Error("Tasks not sorted by default startTime in descending order")
						break
					}
				}
			},
		},
		{
			name:   "List with page number out of range",
			userID: "user-1",
			req: &models.TaskListRequest{
				Page:     100,
				PageSize: 20,
			},
			validate: func(t *testing.T, tasks []*models.ScanTask, total int) {
				if len(tasks) != 0 {
					t.Errorf("Expected empty tasks for out of range page, got %d tasks", len(tasks))
				}
				if total != 5 {
					t.Errorf("Expected total 5 tasks, got %d", total)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, total, err := repo.List(tt.userID, tt.req)
			if err != nil {
				t.Fatalf("Failed to list tasks: %v", err)
			}

			tt.validate(t, tasks, total)
		})
	}
}

// TestGetQueuedTasks tests retrieving queued tasks
func TestGetQueuedTasks(t *testing.T) {
	repo := NewInMemoryScanRepository()

	// Create tasks with different statuses
	statuses := []models.ScanStatus{
		models.ScanStatusQueued,
		models.ScanStatusRunning,
		models.ScanStatusQueued,
		models.ScanStatusCompleted,
		models.ScanStatusQueued,
	}

	for i, status := range statuses {
		task := models.NewScanTask("task-"+string(rune('0'+i)), "user-1", "alpine:latest", &models.ScanConfig{
			TLSVerify: true,
			Format:    "json",
		})
		task.Status = status
		repo.Create(task)
		time.Sleep(10 * time.Millisecond)
	}

	// Get queued tasks
	queued, err := repo.GetQueuedTasks("user-1")
	if err != nil {
		t.Fatalf("Failed to get queued tasks: %v", err)
	}

	// Verify only queued tasks are returned
	expectedQueued := 3
	if len(queued) != expectedQueued {
		t.Errorf("Expected %d queued tasks, got %d", expectedQueued, len(queued))
	}

	for _, task := range queued {
		if task.Status != models.ScanStatusQueued {
			t.Errorf("Expected all tasks to be queued, got %s", task.Status)
		}
	}

	// Verify tasks are ordered by start time
	if len(queued) > 1 {
		for i := 0; i < len(queued)-1; i++ {
			if queued[i].StartTime.After(queued[i+1].StartTime) {
				t.Error("Queued tasks not sorted by start time")
				break
			}
		}
	}
}

// TestConcurrentAccess tests concurrent access to repository
func TestConcurrentAccess(t *testing.T) {
	repo := NewInMemoryScanRepository()

	// Create initial task
	task := models.NewScanTask("task-1", "user-1", "alpine:latest", &models.ScanConfig{
		TLSVerify: true,
		Format:    "json",
	})
	repo.Create(task)

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := repo.GetByID("task-1")
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all reads to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(idx int) {
			task.Message = "Update " + string(rune('0'+idx))
			err := repo.Update(task)
			if err != nil {
				t.Errorf("Concurrent write failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all writes to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify task still exists
	retrieved, err := repo.GetByID("task-1")
	if err != nil {
		t.Fatalf("Failed to retrieve task after concurrent access: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Task should still exist after concurrent access")
	}
}

// TestGetRunningTask tests the GetRunningTask method
func TestGetRunningTask(t *testing.T) {
	repo := NewInMemoryScanRepository()

	// Create tasks with different statuses
	queuedTask := models.NewScanTask("task-1", "user-1", "alpine:latest", &models.ScanConfig{Format: "json"})
	queuedTask.Status = models.ScanStatusQueued
	repo.Create(queuedTask)

	runningTask := models.NewScanTask("task-2", "user-1", "nginx:latest", &models.ScanConfig{Format: "json"})
	runningTask.Status = models.ScanStatusRunning
	repo.Create(runningTask)

	completedTask := models.NewScanTask("task-3", "user-1", "redis:latest", &models.ScanConfig{Format: "json"})
	completedTask.Status = models.ScanStatusCompleted
	repo.Create(completedTask)

	// Test 1: Get running task for user-1
	task, err := repo.GetRunningTask("user-1")
	if err != nil {
		t.Fatalf("GetRunningTask failed: %v", err)
	}
	if task == nil {
		t.Fatal("Expected to find running task, got nil")
	}
	if task.ID != "task-2" {
		t.Errorf("Expected task-2, got %s", task.ID)
	}
	if task.Status != models.ScanStatusRunning {
		t.Errorf("Expected status %s, got %s", models.ScanStatusRunning, task.Status)
	}

	// Test 2: Get running task for user without running tasks
	task2, err := repo.GetRunningTask("user-2")
	if err != nil {
		t.Fatalf("GetRunningTask failed: %v", err)
	}
	if task2 != nil {
		t.Errorf("Expected nil for user without running tasks, got %v", task2)
	}

	// Test 3: After task completes, should return nil
	runningTask.Status = models.ScanStatusCompleted
	repo.Update(runningTask)

	task3, err := repo.GetRunningTask("user-1")
	if err != nil {
		t.Fatalf("GetRunningTask failed: %v", err)
	}
	if task3 != nil {
		t.Errorf("Expected nil after all tasks completed, got %v", task3)
	}
}

// ===== FileBasedScanRepository Tests =====

// TestFileBasedScanRepository_CreateAndGet tests creating and retrieving a task.
func TestFileBasedScanRepository_CreateAndGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "trivy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := NewFileBasedScanRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	task := models.NewScanTask("test-id-1", "user-1", "nginx:latest", &models.ScanConfig{
		Severity:  []string{"HIGH", "CRITICAL"},
		TLSVerify: true,
	})

	err = repo.Create(task)
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}

	metadataPath := filepath.Join(tmpDir, "scans/users/user-1/test-id-1/metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Errorf("Metadata file not created: %s", metadataPath)
	}

	retrieved, err := repo.GetByID("test-id-1")
	if err != nil {
		t.Errorf("GetByID failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetByID returned nil")
	}

	if retrieved.ID != task.ID {
		t.Errorf("Expected ID %s, got %s", task.ID, retrieved.ID)
	}

	if retrieved.UserID != task.UserID {
		t.Errorf("Expected UserID %s, got %s", task.UserID, retrieved.UserID)
	}

	if retrieved.Image != task.Image {
		t.Errorf("Expected Image %s, got %s", task.Image, retrieved.Image)
	}
}

// TestFileBasedScanRepository_SharedDirectory tests tasks without user ID (OIDC disabled).
func TestFileBasedScanRepository_SharedDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "trivy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := NewFileBasedScanRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	task := models.NewScanTask("test-id-shared", "", "nginx:latest", &models.ScanConfig{})

	err = repo.Create(task)
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}

	metadataPath := filepath.Join(tmpDir, "scans/shared/test-id-shared/metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Errorf("Metadata file not created in shared directory: %s", metadataPath)
	}
}

// TestFileBasedScanRepository_Update tests updating a task.
func TestFileBasedScanRepository_Update(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "trivy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := NewFileBasedScanRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	task := models.NewScanTask("test-id-2", "user-1", "nginx:latest", &models.ScanConfig{})
	repo.Create(task)

	task.Status = models.ScanStatusCompleted
	task.Message = "Scan completed"
	endTime := time.Now()
	task.EndTime = &endTime

	err = repo.Update(task)
	if err != nil {
		t.Errorf("Update failed: %v", err)
	}

	retrieved, _ := repo.GetByID("test-id-2")
	if retrieved.Status != models.ScanStatusCompleted {
		t.Errorf("Expected status %s, got %s", models.ScanStatusCompleted, retrieved.Status)
	}

	if retrieved.EndTime == nil {
		t.Error("EndTime should not be nil after update")
	}
}

// TestFileBasedScanRepository_Delete tests deleting a task.
func TestFileBasedScanRepository_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "trivy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := NewFileBasedScanRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	task := models.NewScanTask("test-id-3", "user-1", "nginx:latest", &models.ScanConfig{})
	repo.Create(task)

	err = repo.Delete("test-id-3")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	taskDir := filepath.Join(tmpDir, "scans/users/user-1/test-id-3")
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Errorf("Task directory should be deleted: %s", taskDir)
	}

	retrieved, _ := repo.GetByID("test-id-3")
	if retrieved != nil {
		t.Error("GetByID should return nil after delete")
	}
}

// TestFileBasedScanRepository_Persistence tests that tasks are loaded on startup.
func TestFileBasedScanRepository_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "trivy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo1, err := NewFileBasedScanRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	task1 := models.NewScanTask("persist-1", "user-1", "nginx:latest", &models.ScanConfig{})
	task2 := models.NewScanTask("persist-2", "user-1", "redis:latest", &models.ScanConfig{})

	repo1.Create(task1)
	repo1.Create(task2)

	repo2, err := NewFileBasedScanRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second repository: %v", err)
	}

	retrieved1, _ := repo2.GetByID("persist-1")
	if retrieved1 == nil {
		t.Error("Task persist-1 should be loaded from file system")
	}

	retrieved2, _ := repo2.GetByID("persist-2")
	if retrieved2 == nil {
		t.Error("Task persist-2 should be loaded from file system")
	}

	if retrieved1 != nil && retrieved1.Image != "nginx:latest" {
		t.Errorf("Expected image nginx:latest, got %s", retrieved1.Image)
	}

	if retrieved2 != nil && retrieved2.Image != "redis:latest" {
		t.Errorf("Expected image redis:latest, got %s", retrieved2.Image)
	}
}

// TestFileBasedScanRepository_UserIsolation tests that users can only see their own tasks.
func TestFileBasedScanRepository_UserIsolation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "trivy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := NewFileBasedScanRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	task1 := models.NewScanTask("task-user1", "user-1", "nginx:latest", &models.ScanConfig{})
	task2 := models.NewScanTask("task-user2", "user-2", "redis:latest", &models.ScanConfig{})

	repo.Create(task1)
	repo.Create(task2)

	filter := &models.TaskListRequest{
		Page:     1,
		PageSize: 10,
	}

	tasks, total, _ := repo.List("user-1", filter)
	if total != 1 {
		t.Errorf("User-1 should see 1 task, got %d", total)
	}

	if len(tasks) > 0 && tasks[0].ID != "task-user1" {
		t.Errorf("User-1 should only see their own task")
	}

	tasks, total, _ = repo.List("user-2", filter)
	if total != 1 {
		t.Errorf("User-2 should see 1 task, got %d", total)
	}

	if len(tasks) > 0 && tasks[0].ID != "task-user2" {
		t.Errorf("User-2 should only see their own task")
	}
}
