// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lazycatapps/trivy/backend/internal/models"
	"github.com/lazycatapps/trivy/backend/internal/repository"
	"github.com/lazycatapps/trivy/backend/internal/types"
)

// mockLogger implements logger.Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Info(format string, args ...interface{})  {}
func (m *mockLogger) Error(format string, args ...interface{}) {}
func (m *mockLogger) Debug(format string, args ...interface{}) {}

// mockCommandExecutor implements CommandExecutor interface for testing
type mockCommandExecutor struct {
	mockStdout string
	mockStderr string
	mockError  error
}

func (m *mockCommandExecutor) ExecuteCommand(ctx context.Context, name string, args []string, logCallback func(string)) (string, string, error) {
	if m.mockError != nil {
		return "", m.mockStderr, m.mockError
	}
	// Simulate streaming logs
	if logCallback != nil {
		for _, line := range []string{
			"Scan started",
			"Analyzing vulnerabilities...",
			"Scan completed",
		} {
			logCallback(line)
		}
	}
	return m.mockStdout, m.mockStderr, nil
}

// createMockJSONOutput creates a valid Trivy JSON output for testing
func createMockJSONOutput() string {
	output := map[string]interface{}{
		"Results": []map[string]interface{}{
			{
				"Vulnerabilities": []map[string]interface{}{
					{"Severity": "CRITICAL"},
					{"Severity": "HIGH"},
					{"Severity": "MEDIUM"},
				},
			},
		},
	}
	data, _ := json.Marshal(output)
	return string(data)
}

// TestNewScanService tests ScanService creation
func TestNewScanService(t *testing.T) {
	repo := repository.NewInMemoryScanRepository()
	config := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 3,
	}
	logger := &mockLogger{}
	storageDir := "/tmp/trivy-test"

	service := NewScanService(repo, config, storageDir, logger)
	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	// Test default max workers
	config2 := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 0, // Should default to 5
	}
	service2 := NewScanService(repo, config2, storageDir, logger)
	if service2 == nil {
		t.Fatal("Expected non-nil service with default max workers")
	}
}

// TestCreateScanTask tests creating a scan task
func TestCreateScanTask(t *testing.T) {
	repo := repository.NewInMemoryScanRepository()
	config := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 5,
	}
	logger := &mockLogger{}
	storageDir := t.TempDir()
	mockExecutor := &mockCommandExecutor{
		mockStdout: createMockJSONOutput(),
	}

	tests := []struct {
		name      string
		userID    string
		req       *models.ScanRequest
		wantError bool
	}{
		{
			name:   "Valid scan request with defaults",
			userID: "user1",
			req: &models.ScanRequest{
				Image: "alpine:latest",
			},
			wantError: false,
		},
		{
			name:   "Valid scan request with all options",
			userID: "user2",
			req: &models.ScanRequest{
				Image:             "ubuntu:22.04",
				Username:          "testuser",
				Password:          "testpass",
				TLSVerify:         boolPtr(true),
				Severity:          []string{"HIGH", "CRITICAL"},
				IgnoreUnfixed:     true,
				Scanners:          []string{"vuln", "secret"},
				DetectionPriority: "comprehensive",
				PkgTypes:          []string{"os"},
				Format:            "json",
			},
			wantError: false,
		},
		{
			name:   "Valid scan request with TLS verify false",
			userID: "user3",
			req: &models.ScanRequest{
				Image:     "nginx:latest",
				TLSVerify: boolPtr(false),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new service for each test to avoid channel close issues
			testService := NewScanServiceWithExecutor(repo, config, storageDir, logger, mockExecutor)
			defer testService.Stop()

			task, err := testService.CreateScanTask(tt.userID, tt.req)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if task == nil {
				t.Fatal("Expected non-nil task")
			}

			// Verify task fields
			if task.ID == "" {
				t.Error("Expected non-empty task ID")
			}

			if task.UserID != tt.userID {
				t.Errorf("Expected userID %s, got %s", tt.userID, task.UserID)
			}

			if task.Image != tt.req.Image {
				t.Errorf("Expected image %s, got %s", tt.req.Image, task.Image)
			}

			if task.Status != models.ScanStatusQueued && task.Status != models.ScanStatusRunning {
				t.Errorf("Expected status queued or running, got %s", task.Status)
			}

			// Verify default values are set
			if tt.req.TLSVerify == nil && !task.ScanConfig.TLSVerify {
				t.Error("Expected TLSVerify to default to true")
			}

			if len(tt.req.Scanners) == 0 && len(task.ScanConfig.Scanners) == 0 {
				t.Error("Expected default scanners to be set")
			}

			if tt.req.DetectionPriority == "" && task.ScanConfig.DetectionPriority == "" {
				t.Error("Expected default detection priority to be set")
			}

			if tt.req.Format == "" && task.ScanConfig.Format == "" {
				t.Error("Expected default format to be set")
			}
		})
	}
}

// TestGetTask tests retrieving a scan task
func TestGetTask(t *testing.T) {
	repo := repository.NewInMemoryScanRepository()
	config := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 5,
	}
	logger := &mockLogger{}
	storageDir := "/tmp/trivy-test"

	service := NewScanService(repo, config, storageDir, logger)
	defer service.Stop()

	// Create a task first
	req := &models.ScanRequest{
		Image: "alpine:latest",
	}
	task, err := service.CreateScanTask("user1", req)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Test getting existing task
	retrieved, err := service.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, retrieved.ID)
	}

	// Test getting non-existent task
	_, err = service.GetTask("non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent task")
	}
}

// TestListTasks tests listing scan tasks
func TestListTasks(t *testing.T) {
	repo := repository.NewInMemoryScanRepository()
	config := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 5,
	}
	logger := &mockLogger{}
	storageDir := "/tmp/trivy-test"

	service := NewScanService(repo, config, storageDir, logger)
	defer service.Stop()

	// Create multiple tasks
	for i := 0; i < 5; i++ {
		req := &models.ScanRequest{
			Image: "alpine:latest",
		}
		_, err := service.CreateScanTask("user1", req)
		if err != nil {
			t.Fatalf("Failed to create task %d: %v", i, err)
		}
	}

	// Wait a bit for tasks to settle
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name     string
		userID   string
		req      *models.TaskListRequest
		validate func(*testing.T, *models.TaskListResponse)
	}{
		{
			name:   "List all tasks with defaults",
			userID: "user1",
			req:    &models.TaskListRequest{},
			validate: func(t *testing.T, resp *models.TaskListResponse) {
				if resp.Total < 5 {
					t.Errorf("Expected at least 5 tasks, got %d", resp.Total)
				}
				if resp.Page != 1 {
					t.Errorf("Expected page 1, got %d", resp.Page)
				}
				if resp.PageSize != 20 {
					t.Errorf("Expected pageSize 20, got %d", resp.PageSize)
				}
			},
		},
		{
			name:   "List with pagination",
			userID: "user1",
			req: &models.TaskListRequest{
				Page:     1,
				PageSize: 2,
			},
			validate: func(t *testing.T, resp *models.TaskListResponse) {
				if len(resp.Tasks) > 2 {
					t.Errorf("Expected max 2 tasks, got %d", len(resp.Tasks))
				}
			},
		},
		{
			name:   "List with sorting",
			userID: "user1",
			req: &models.TaskListRequest{
				SortBy:    "startTime",
				SortOrder: "desc",
			},
			validate: func(t *testing.T, resp *models.TaskListResponse) {
				if len(resp.Tasks) < 2 {
					return
				}
				// Verify descending order
				for i := 0; i < len(resp.Tasks)-1; i++ {
					if resp.Tasks[i].StartTime.Before(resp.Tasks[i+1].StartTime) {
						t.Error("Tasks not sorted in descending order")
						break
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.ListTasks(tt.userID, tt.req)
			if err != nil {
				t.Fatalf("Failed to list tasks: %v", err)
			}

			if resp == nil {
				t.Fatal("Expected non-nil response")
			}

			tt.validate(t, resp)
		})
	}
}

// TestGetQueueStatus tests getting queue status
func TestGetQueueStatus(t *testing.T) {
	repo := repository.NewInMemoryScanRepository()
	config := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 5,
	}
	logger := &mockLogger{}
	storageDir := "/tmp/trivy-test"

	service := NewScanService(repo, config, storageDir, logger)
	defer service.Stop()

	// Get queue status
	status, err := service.GetQueueStatus("user1")
	if err != nil {
		t.Fatalf("Failed to get queue status: %v", err)
	}

	if status == nil {
		t.Fatal("Expected non-nil status")
	}

	if status.QueueLength < 0 {
		t.Error("Expected non-negative queue length")
	}

	if status.AverageWaitTime < 0 {
		t.Error("Expected non-negative average wait time")
	}
}

// TestBuildTrivyArgs tests building trivy command arguments
func TestBuildTrivyArgs(t *testing.T) {
	config := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 5,
	}
	logger := &mockLogger{}
	repo := repository.NewInMemoryScanRepository()
	storageDir := "/tmp/trivy-test"

	service := NewScanService(repo, config, storageDir, logger).(*scanServiceImpl)
	defer service.Stop()

	tests := []struct {
		name        string
		task        *models.ScanTask
		contains    []string
		notContains []string
	}{
		{
			name: "Basic scan",
			task: &models.ScanTask{
				Image: "alpine:latest",
				ScanConfig: &models.ScanConfig{
					TLSVerify:         true,
					Scanners:          []string{"vuln"},
					DetectionPriority: "precise",
					Format:            "json",
				},
			},
			contains:    []string{"image", "--server", "http://localhost:4954", "--format", "json", "--scanners", "vuln", "--detection-priority", "precise", "alpine:latest"},
			notContains: []string{"--insecure"},
		},
		{
			name: "Scan with insecure",
			task: &models.ScanTask{
				Image: "nginx:latest",
				ScanConfig: &models.ScanConfig{
					TLSVerify: false,
					Format:    "json",
				},
			},
			contains: []string{"image", "--insecure", "nginx:latest"},
		},
		{
			name: "Scan with credentials",
			task: &models.ScanTask{
				Image: "private.registry/image:tag",
				ScanConfig: &models.ScanConfig{
					Username:  "testuser",
					Password:  "testpass",
					TLSVerify: true,
					Format:    "json",
				},
			},
			contains: []string{"--username", "testuser", "--password", "testpass"},
		},
		{
			name: "Scan with severity filter",
			task: &models.ScanTask{
				Image: "ubuntu:22.04",
				ScanConfig: &models.ScanConfig{
					Severity:  []string{"HIGH", "CRITICAL"},
					TLSVerify: true,
					Format:    "json",
				},
			},
			contains: []string{"--severity", "HIGH,CRITICAL"},
		},
		{
			name: "Scan with ignore unfixed",
			task: &models.ScanTask{
				Image: "debian:11",
				ScanConfig: &models.ScanConfig{
					IgnoreUnfixed: true,
					TLSVerify:     true,
					Format:        "json",
				},
			},
			contains: []string{"--ignore-unfixed"},
		},
		{
			name: "Scan with multiple scanners",
			task: &models.ScanTask{
				Image: "node:18",
				ScanConfig: &models.ScanConfig{
					Scanners:  []string{"vuln", "secret", "misconfig"},
					TLSVerify: true,
					Format:    "json",
				},
			},
			contains: []string{"--scanners", "vuln,secret,misconfig"},
		},
		{
			name: "Scan with pkg types",
			task: &models.ScanTask{
				Image: "python:3.11",
				ScanConfig: &models.ScanConfig{
					PkgTypes:  []string{"os", "library"},
					TLSVerify: true,
					Format:    "json",
				},
			},
			contains: []string{"--pkg-types", "os,library"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := service.buildTrivyArgs(tt.task)

			// Convert to string for easier checking
			argsStr := ""
			for _, arg := range args {
				argsStr += arg + " "
			}

			for _, contain := range tt.contains {
				found := false
				for _, arg := range args {
					if arg == contain {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected args to contain '%s', got: %v", contain, args)
				}
			}

			for _, notContain := range tt.notContains {
				for _, arg := range args {
					if arg == notContain {
						t.Errorf("Expected args not to contain '%s', got: %v", notContain, args)
					}
				}
			}
		})
	}
}

// TestMaskCredentials tests masking credentials in command arguments
func TestMaskCredentials(t *testing.T) {
	config := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 5,
	}
	logger := &mockLogger{}
	repo := repository.NewInMemoryScanRepository()
	storageDir := "/tmp/trivy-test"

	service := NewScanService(repo, config, storageDir, logger).(*scanServiceImpl)
	defer service.Stop()

	args := []string{
		"image",
		"--username", "myuser",
		"--password", "mypassword",
		"--server", "http://localhost:4954",
		"alpine:latest",
	}

	masked := service.maskCredentials(args)

	// Check that credentials are masked
	for i, arg := range masked {
		if arg == "--username" && i+1 < len(masked) {
			if masked[i+1] != "***" {
				t.Errorf("Expected username to be masked, got %s", masked[i+1])
			}
		}
		if arg == "--password" && i+1 < len(masked) {
			if masked[i+1] != "***" {
				t.Errorf("Expected password to be masked, got %s", masked[i+1])
			}
		}
	}

	// Check that other arguments are not masked
	if masked[0] != "image" {
		t.Errorf("Expected first arg to be 'image', got %s", masked[0])
	}
}

// TestParseVulnerabilitySummary tests parsing vulnerability summary from JSON
func TestParseVulnerabilitySummary(t *testing.T) {
	config := &types.TrivyConfig{
		ServerURL:  "http://localhost:4954",
		Timeout:    600,
		MaxWorkers: 5,
	}
	logger := &mockLogger{}
	repo := repository.NewInMemoryScanRepository()
	storageDir := "/tmp/trivy-test"

	service := NewScanService(repo, config, storageDir, logger).(*scanServiceImpl)
	defer service.Stop()

	tests := []struct {
		name       string
		jsonOutput string
		want       *models.VulnerabilitySummary
		wantError  bool
	}{
		{
			name: "Valid JSON with vulnerabilities",
			jsonOutput: `{
				"Results": [
					{
						"Vulnerabilities": [
							{"Severity": "CRITICAL"},
							{"Severity": "HIGH"},
							{"Severity": "HIGH"},
							{"Severity": "MEDIUM"},
							{"Severity": "LOW"},
							{"Severity": "UNKNOWN"}
						]
					}
				]
			}`,
			want: &models.VulnerabilitySummary{
				Total:    6,
				Critical: 1,
				High:     2,
				Medium:   1,
				Low:      1,
				Unknown:  1,
			},
			wantError: false,
		},
		{
			name: "Empty results",
			jsonOutput: `{
				"Results": []
			}`,
			want: &models.VulnerabilitySummary{
				Total:    0,
				Critical: 0,
				High:     0,
				Medium:   0,
				Low:      0,
				Unknown:  0,
			},
			wantError: false,
		},
		{
			name:       "Invalid JSON",
			jsonOutput: `invalid json`,
			want:       nil,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := service.parseVulnerabilitySummary(tt.jsonOutput)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if summary.Total != tt.want.Total {
				t.Errorf("Expected total %d, got %d", tt.want.Total, summary.Total)
			}
			if summary.Critical != tt.want.Critical {
				t.Errorf("Expected critical %d, got %d", tt.want.Critical, summary.Critical)
			}
			if summary.High != tt.want.High {
				t.Errorf("Expected high %d, got %d", tt.want.High, summary.High)
			}
			if summary.Medium != tt.want.Medium {
				t.Errorf("Expected medium %d, got %d", tt.want.Medium, summary.Medium)
			}
			if summary.Low != tt.want.Low {
				t.Errorf("Expected low %d, got %d", tt.want.Low, summary.Low)
			}
			if summary.Unknown != tt.want.Unknown {
				t.Errorf("Expected unknown %d, got %d", tt.want.Unknown, summary.Unknown)
			}
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
