// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package models

import (
	"sync"
	"testing"
	"time"
)

// TestNewScanTask tests the NewScanTask constructor
func TestNewScanTask(t *testing.T) {
	config := &ScanConfig{
		Severity: []string{"HIGH", "CRITICAL"},
		Scanners: []string{"vuln"},
	}

	task := NewScanTask("task-123", "user-1", "alpine:latest", config)

	if task.ID != "task-123" {
		t.Errorf("Expected ID 'task-123', got '%s'", task.ID)
	}

	if task.UserID != "user-1" {
		t.Errorf("Expected UserID 'user-1', got '%s'", task.UserID)
	}

	if task.Image != "alpine:latest" {
		t.Errorf("Expected Image 'alpine:latest', got '%s'", task.Image)
	}

	if task.Status != ScanStatusQueued {
		t.Errorf("Expected Status 'queued', got '%s'", task.Status)
	}

	if task.Message != "Task created and queued" {
		t.Errorf("Expected Message 'Task created and queued', got '%s'", task.Message)
	}

	if task.ScanConfig != config {
		t.Error("Expected ScanConfig to be set")
	}

	if task.LogLines == nil {
		t.Error("Expected LogLines to be initialized")
	}

	if task.LogListeners == nil {
		t.Error("Expected LogListeners to be initialized")
	}

	if time.Since(task.StartTime) > time.Second {
		t.Error("Expected StartTime to be recent")
	}
}

// TestAddLog tests adding log lines
func TestAddLog(t *testing.T) {
	task := NewScanTask("task-1", "user-1", "alpine:latest", &ScanConfig{})

	task.AddLog("Line 1")
	task.AddLog("Line 2")
	task.AddLog("Line 3")

	logs := task.GetLogLines()
	if len(logs) != 3 {
		t.Errorf("Expected 3 log lines, got %d", len(logs))
	}

	if logs[0] != "Line 1" || logs[1] != "Line 2" || logs[2] != "Line 3" {
		t.Errorf("Log lines don't match expected values: %v", logs)
	}
}

// TestAddLogListener tests adding and receiving from log listeners
func TestAddLogListener(t *testing.T) {
	task := NewScanTask("task-1", "user-1", "alpine:latest", &ScanConfig{})

	// Add a listener
	listener := task.AddLogListener()

	// Add logs
	task.AddLog("Test message 1")
	task.AddLog("Test message 2")

	// Receive logs from listener
	select {
	case msg := <-listener:
		if msg != "Test message 1" {
			t.Errorf("Expected 'Test message 1', got '%s'", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for log message")
	}

	select {
	case msg := <-listener:
		if msg != "Test message 2" {
			t.Errorf("Expected 'Test message 2', got '%s'", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for log message")
	}
}

// TestRemoveLogListener tests removing a log listener
func TestRemoveLogListener(t *testing.T) {
	task := NewScanTask("task-1", "user-1", "alpine:latest", &ScanConfig{})

	listener := task.AddLogListener()

	// Remove the listener
	task.RemoveLogListener(listener)

	// Add a log (should not be received by removed listener)
	task.AddLog("After removal")

	// Listener channel should be closed
	select {
	case _, ok := <-listener:
		if ok {
			t.Error("Expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for channel close")
	}
}

// TestCloseAllLogListeners tests closing all listeners
func TestCloseAllLogListeners(t *testing.T) {
	task := NewScanTask("task-1", "user-1", "alpine:latest", &ScanConfig{})

	listener1 := task.AddLogListener()
	listener2 := task.AddLogListener()

	task.CloseAllLogListeners()

	// Both channels should be closed
	_, ok1 := <-listener1
	_, ok2 := <-listener2

	if ok1 || ok2 {
		t.Error("Expected all listener channels to be closed")
	}

	// LogListeners should be empty
	if len(task.LogListeners) != 0 {
		t.Errorf("Expected 0 listeners, got %d", len(task.LogListeners))
	}
}

// TestGetLogLines tests getting log lines copy
func TestGetLogLines(t *testing.T) {
	task := NewScanTask("task-1", "user-1", "alpine:latest", &ScanConfig{})

	task.AddLog("Line 1")
	task.AddLog("Line 2")

	logs := task.GetLogLines()

	// Modify the copy
	logs[0] = "Modified"

	// Original should remain unchanged
	originalLogs := task.GetLogLines()
	if originalLogs[0] != "Line 1" {
		t.Error("GetLogLines should return a copy, not a reference")
	}
}

// TestConcurrentAddLog tests concurrent log additions
func TestConcurrentAddLog(t *testing.T) {
	task := NewScanTask("task-1", "user-1", "alpine:latest", &ScanConfig{})

	var wg sync.WaitGroup
	numGoroutines := 10
	logsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				task.AddLog("Log from goroutine")
			}
		}(i)
	}

	wg.Wait()

	logs := task.GetLogLines()
	expectedCount := numGoroutines * logsPerGoroutine
	if len(logs) != expectedCount {
		t.Errorf("Expected %d log lines, got %d", expectedCount, len(logs))
	}
}

// TestToSummary tests converting task to summary
func TestToSummary(t *testing.T) {
	endTime := time.Now()
	task := &ScanTask{
		ID:            "task-123",
		Image:         "nginx:latest",
		Status:        ScanStatusCompleted,
		Message:       "Scan completed",
		StartTime:     time.Now().Add(-5 * time.Minute),
		EndTime:       &endTime,
		QueuePosition: 0,
		Result: &ScanResult{
			Summary: &VulnerabilitySummary{
				Total:    10,
				Critical: 2,
				High:     3,
				Medium:   4,
				Low:      1,
			},
		},
	}

	summary := task.ToSummary()

	if summary.ID != "task-123" {
		t.Errorf("Expected ID 'task-123', got '%s'", summary.ID)
	}

	if summary.Image != "nginx:latest" {
		t.Errorf("Expected Image 'nginx:latest', got '%s'", summary.Image)
	}

	if summary.Status != "completed" {
		t.Errorf("Expected Status 'completed', got '%s'", summary.Status)
	}

	if summary.Summary == nil {
		t.Error("Expected Summary to be set")
	} else {
		if summary.Summary.Total != 10 {
			t.Errorf("Expected Total 10, got %d", summary.Summary.Total)
		}
		if summary.Summary.Critical != 2 {
			t.Errorf("Expected Critical 2, got %d", summary.Summary.Critical)
		}
	}
}

// TestToSummaryWithoutResult tests converting task without result
func TestToSummaryWithoutResult(t *testing.T) {
	task := &ScanTask{
		ID:        "task-456",
		Image:     "alpine:latest",
		Status:    ScanStatusQueued,
		Message:   "Task in queue",
		StartTime: time.Now(),
	}

	summary := task.ToSummary()

	if summary.Summary != nil {
		t.Error("Expected Summary to be nil when task has no result")
	}
}

// TestLogListenerBufferOverflow tests that full listener channels are skipped
func TestLogListenerBufferOverflow(t *testing.T) {
	task := NewScanTask("task-1", "user-1", "alpine:latest", &ScanConfig{})

	// Create a listener but don't read from it
	listener := task.AddLogListener()

	// Fill the buffer (channel size is 100)
	for i := 0; i < 150; i++ {
		task.AddLog("Message")
	}

	// Should not panic or block
	// Listener should have 100 messages (buffer size)
	count := 0
	for {
		select {
		case <-listener:
			count++
		case <-time.After(10 * time.Millisecond):
			// No more messages
			if count != 100 {
				t.Errorf("Expected 100 messages in buffer, got %d", count)
			}
			return
		}
	}
}
