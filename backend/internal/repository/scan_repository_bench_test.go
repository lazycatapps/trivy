// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package repository

import (
	"fmt"
	"testing"

	"github.com/lazycatapps/trivy/backend/internal/models"
)

// BenchmarkCreate measures the performance of creating scan tasks
func BenchmarkCreate(b *testing.B) {
	repo := NewInMemoryScanRepository()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
		repo.Create(task)
	}
}

// BenchmarkGetByID measures the performance of retrieving tasks by ID
func BenchmarkGetByID(b *testing.B) {
	repo := NewInMemoryScanRepository()

	// Setup: Create 1000 tasks
	for i := 0; i < 1000; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
		repo.Create(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := fmt.Sprintf("task-%d", i%1000)
		repo.GetByID(taskID)
	}
}

// BenchmarkUpdate measures the performance of updating tasks
func BenchmarkUpdate(b *testing.B) {
	repo := NewInMemoryScanRepository()

	// Setup: Create 1000 tasks
	tasks := make([]*models.ScanTask, 1000)
	for i := 0; i < 1000; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
		repo.Create(task)
		tasks[i] = task
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := tasks[i%1000]
		task.Status = models.ScanStatusCompleted
		task.Message = "Benchmark update"
		repo.Update(task)
	}
}

// BenchmarkList measures the performance of listing tasks with pagination
func BenchmarkList(b *testing.B) {
	repo := NewInMemoryScanRepository()

	// Setup: Create 10000 tasks across 10 users
	for i := 0; i < 10000; i++ {
		userID := fmt.Sprintf("user-%d", i%10)
		taskID := fmt.Sprintf("task-%d", i)
		task := models.NewScanTask(taskID, userID, "alpine:latest", &models.ScanConfig{})
		repo.Create(task)
	}

	req := &models.TaskListRequest{
		Page:      1,
		PageSize:  20,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := fmt.Sprintf("user-%d", i%10)
		repo.List(userID, req)
	}
}

// BenchmarkListWithFilter measures the performance of listing tasks with status filter
func BenchmarkListWithFilter(b *testing.B) {
	repo := NewInMemoryScanRepository()

	// Setup: Create 10000 tasks with different statuses
	statuses := []models.ScanStatus{
		models.ScanStatusQueued,
		models.ScanStatusRunning,
		models.ScanStatusCompleted,
		models.ScanStatusFailed,
	}

	for i := 0; i < 10000; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
		task.Status = statuses[i%len(statuses)]
		repo.Create(task)
	}

	req := &models.TaskListRequest{
		Page:      1,
		PageSize:  20,
		Status:    string(models.ScanStatusCompleted),
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.List("user-1", req)
	}
}

// BenchmarkGetQueuedTasks measures the performance of retrieving queued tasks
func BenchmarkGetQueuedTasks(b *testing.B) {
	repo := NewInMemoryScanRepository()

	// Setup: Create mix of queued and non-queued tasks
	for i := 0; i < 5000; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
		if i%2 == 0 {
			task.Status = models.ScanStatusQueued
		} else {
			task.Status = models.ScanStatusCompleted
		}
		repo.Create(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.GetQueuedTasks("user-1")
	}
}

// BenchmarkConcurrentRead measures concurrent read performance
func BenchmarkConcurrentRead(b *testing.B) {
	repo := NewInMemoryScanRepository()

	// Setup: Create 1000 tasks
	for i := 0; i < 1000; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
		repo.Create(task)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			taskID := fmt.Sprintf("task-%d", i%1000)
			repo.GetByID(taskID)
			i++
		}
	})
}

// BenchmarkConcurrentWrite measures concurrent write performance
func BenchmarkConcurrentWrite(b *testing.B) {
	repo := NewInMemoryScanRepository()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			taskID := fmt.Sprintf("task-%d-%d", b.N, i)
			task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
			repo.Create(task)
			i++
		}
	})
}

// BenchmarkConcurrentMixed measures mixed read/write performance
func BenchmarkConcurrentMixed(b *testing.B) {
	repo := NewInMemoryScanRepository()

	// Setup: Create initial tasks
	for i := 0; i < 1000; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
		repo.Create(task)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				// Read operation
				taskID := fmt.Sprintf("task-%d", i%1000)
				repo.GetByID(taskID)
			} else {
				// Write operation
				taskID := fmt.Sprintf("task-new-%d-%d", b.N, i)
				task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
				repo.Create(task)
			}
			i++
		}
	})
}
