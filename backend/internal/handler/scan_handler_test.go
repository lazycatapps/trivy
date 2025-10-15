// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lazycatapps/trivy/backend/internal/models"
)

// mockScanService implements service.ScanService for testing
type mockScanService struct {
	createTaskFunc     func(userID string, req *models.ScanRequest) (*models.ScanTask, error)
	getTaskFunc        func(taskID string) (*models.ScanTask, error)
	listTasksFunc      func(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error)
	getQueueStatusFunc func(userID string) (*models.QueueStatusResponse, error)
	getTrivyVersionFunc func(ctx context.Context) (*models.TrivyVersion, error)
}

func (m *mockScanService) CreateScanTask(userID string, req *models.ScanRequest) (*models.ScanTask, error) {
	if m.createTaskFunc != nil {
		return m.createTaskFunc(userID, req)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockScanService) GetTask(taskID string) (*models.ScanTask, error) {
	if m.getTaskFunc != nil {
		return m.getTaskFunc(taskID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockScanService) ListTasks(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error) {
	if m.listTasksFunc != nil {
		return m.listTasksFunc(userID, req)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockScanService) GetQueueStatus(userID string) (*models.QueueStatusResponse, error) {
	if m.getQueueStatusFunc != nil {
		return m.getQueueStatusFunc(userID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockScanService) GetTrivyVersion(ctx context.Context) (*models.TrivyVersion, error) {
	if m.getTrivyVersionFunc != nil {
		return m.getTrivyVersionFunc(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockScanService) ListDockerImages() ([]models.DockerImage, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockScanService) ListDockerContainers() ([]models.DockerContainer, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockScanService) DeleteTask(taskID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockScanService) DeleteAllTasks(userID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockScanService) Start() {}
func (m *mockScanService) Stop()  {}

// mockLogger implements logger.Logger for testing
type mockLogger struct{}

func (m *mockLogger) Info(format string, args ...interface{})  {}
func (m *mockLogger) Error(format string, args ...interface{}) {}
func (m *mockLogger) Debug(format string, args ...interface{}) {}

// setupTestRouter creates a test Gin router
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// TestCreateScan tests the CreateScan handler
func TestCreateScan(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		userID         string
		setUserID      bool // Whether to set userID in context
		mockCreateTask func(userID string, req *models.ScanRequest) (*models.ScanTask, error)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "Valid scan request",
			requestBody: map[string]interface{}{
				"image":    "alpine:latest",
				"scanners": []string{"vuln"},
			},
			userID:    "test-user",
			setUserID: true,
			mockCreateTask: func(userID string, req *models.ScanRequest) (*models.ScanTask, error) {
				return models.NewScanTask("task-123", userID, req.Image, &models.ScanConfig{}), nil
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["id"] != "task-123" {
					t.Errorf("Expected task ID 'task-123', got %v", resp["id"])
				}
				if resp["message"] != "Scan started" {
					t.Errorf("Expected message 'Scan started', got %v", resp["message"])
				}
			},
		},
		{
			name: "Missing image field",
			requestBody: map[string]interface{}{
				"image":    "",
				"scanners": []string{"vuln"},
			},
			userID:         "test-user",
			setUserID:      true,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if _, ok := resp["error"]; !ok {
					t.Error("Expected error field in response")
				}
				// Note: Gin's binding validation catches empty image field
				// The error message will be from the validator, not the manual check
			},
		},
		{
			name:           "Invalid JSON body",
			requestBody:    "invalid json",
			userID:         "test-user",
			setUserID:      true,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if _, ok := resp["error"]; !ok {
					t.Error("Expected error field in response")
				}
			},
		},
		{
			name: "Service error",
			requestBody: map[string]interface{}{
				"image": "alpine:latest",
			},
			userID:    "test-user",
			setUserID: true,
			mockCreateTask: func(userID string, req *models.ScanRequest) (*models.ScanTask, error) {
				return nil, fmt.Errorf("service error")
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if _, ok := resp["error"]; !ok {
					t.Error("Expected error field in response")
				}
			},
		},
		{
			name: "Anonymous user - userID not set in context",
			requestBody: map[string]interface{}{
				"image":    "nginx:latest",
				"scanners": []string{"vuln"},
			},
			userID:    "",
			setUserID: false, // Don't set userID in context
			mockCreateTask: func(userID string, req *models.ScanRequest) (*models.ScanTask, error) {
				// Should be called with "anonymous"
				if userID != "anonymous" {
					return nil, fmt.Errorf("expected userID 'anonymous', got '%s'", userID)
				}
				return models.NewScanTask("task-anonymous", userID, req.Image, &models.ScanConfig{}), nil
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["id"] != "task-anonymous" {
					t.Errorf("Expected task ID 'task-anonymous', got %v", resp["id"])
				}
				if resp["message"] != "Scan started" {
					t.Errorf("Expected message 'Scan started', got %v", resp["message"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockScanService{
				createTaskFunc: tt.mockCreateTask,
			}
			handler := NewScanHandler(mockService, &mockLogger{})
			router := setupTestRouter()

			router.POST("/scan", func(c *gin.Context) {
				if tt.setUserID {
					c.Set("userID", tt.userID)
				}
				handler.CreateScan(c)
			})

			// Create request
			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, _ = json.Marshal(tt.requestBody)
			}

			req := httptest.NewRequest(http.MethodPost, "/scan", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Assert response body
			if tt.checkResponse != nil {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				tt.checkResponse(t, response)
			}
		})
	}
}

// TestGetScan tests the GetScan handler
func TestGetScan(t *testing.T) {
	tests := []struct {
		name           string
		taskID         string
		mockGetTask    func(taskID string) (*models.ScanTask, error)
		expectedStatus int
		checkResponse  func(*testing.T, *models.ScanTask)
	}{
		{
			name:   "Get existing task",
			taskID: "task-123",
			mockGetTask: func(taskID string) (*models.ScanTask, error) {
				task := models.NewScanTask(taskID, "user-1", "alpine:latest", &models.ScanConfig{})
				task.Status = models.ScanStatusCompleted
				return task, nil
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, task *models.ScanTask) {
				if task.ID != "task-123" {
					t.Errorf("Expected task ID 'task-123', got %s", task.ID)
				}
				if task.Status != models.ScanStatusCompleted {
					t.Errorf("Expected status completed, got %s", task.Status)
				}
			},
		},
		{
			name:   "Task not found",
			taskID: "non-existent",
			mockGetTask: func(taskID string) (*models.ScanTask, error) {
				return nil, fmt.Errorf("task not found")
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockScanService{
				getTaskFunc: tt.mockGetTask,
			}
			handler := NewScanHandler(mockService, &mockLogger{})
			router := setupTestRouter()
			router.GET("/scan/:id", handler.GetScan)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/scan/"+tt.taskID, nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Assert response
			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var task models.ScanTask
				if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				tt.checkResponse(t, &task)
			}
		})
	}
}

// TestListScans tests the ListScans handler
func TestListScans(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		userID         string
		setUserID      bool // Whether to set userID in context
		mockListTasks  func(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error)
		expectedStatus int
		checkResponse  func(*testing.T, *models.TaskListResponse)
	}{
		{
			name:        "List tasks with defaults",
			queryParams: "",
			userID:      "test-user",
			setUserID:   true,
			mockListTasks: func(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error) {
				return &models.TaskListResponse{
					Total:    5,
					Page:     1,
					PageSize: 20,
					Tasks:    []*models.TaskSummary{},
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *models.TaskListResponse) {
				if resp.Total != 5 {
					t.Errorf("Expected total 5, got %d", resp.Total)
				}
			},
		},
		{
			name:        "List tasks with pagination",
			queryParams: "?page=2&pageSize=10",
			userID:      "test-user",
			setUserID:   true,
			mockListTasks: func(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error) {
				if req.Page != 2 || req.PageSize != 10 {
					t.Errorf("Expected page=2 pageSize=10, got page=%d pageSize=%d", req.Page, req.PageSize)
				}
				return &models.TaskListResponse{
					Total:    25,
					Page:     2,
					PageSize: 10,
					Tasks:    []*models.TaskSummary{},
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *models.TaskListResponse) {
				if resp.Page != 2 || resp.PageSize != 10 {
					t.Errorf("Expected page=2 pageSize=10, got page=%d pageSize=%d", resp.Page, resp.PageSize)
				}
			},
		},
		{
			name:        "Service error",
			queryParams: "",
			userID:      "test-user",
			setUserID:   true,
			mockListTasks: func(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error) {
				return nil, fmt.Errorf("service error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "Anonymous user - userID not set in context",
			queryParams: "",
			userID:      "",
			setUserID:   false, // Don't set userID in context
			mockListTasks: func(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error) {
				// Should be called with "anonymous"
				if userID != "anonymous" {
					return nil, fmt.Errorf("expected userID 'anonymous', got '%s'", userID)
				}
				return &models.TaskListResponse{
					Total:    3,
					Page:     1,
					PageSize: 20,
					Tasks:    []*models.TaskSummary{},
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *models.TaskListResponse) {
				if resp.Total != 3 {
					t.Errorf("Expected total 3, got %d", resp.Total)
				}
			},
		},
		{
			name:        "Invalid query parameters",
			queryParams: "?page=invalid",
			userID:      "test-user",
			setUserID:   true,
			mockListTasks: func(userID string, req *models.TaskListRequest) (*models.TaskListResponse, error) {
				// Should not be called
				t.Error("ListTasks should not be called with invalid parameters")
				return nil, nil
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockScanService{
				listTasksFunc: tt.mockListTasks,
			}
			handler := NewScanHandler(mockService, &mockLogger{})
			router := setupTestRouter()

			router.GET("/scan", func(c *gin.Context) {
				if tt.setUserID {
					c.Set("userID", tt.userID)
				}
				handler.ListScans(c)
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/scan"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Assert response
			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var response models.TaskListResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				tt.checkResponse(t, &response)
			}
		})
	}
}

// TestGetQueueStatus tests the GetQueueStatus handler
func TestGetQueueStatus(t *testing.T) {
	tests := []struct {
		name               string
		userID             string
		setUserID          bool // Whether to set userID in context
		mockGetQueueStatus func(userID string) (*models.QueueStatusResponse, error)
		expectedStatus     int
		checkResponse      func(*testing.T, *models.QueueStatusResponse)
	}{
		{
			name:      "Get queue status successfully",
			userID:    "test-user",
			setUserID: true,
			mockGetQueueStatus: func(userID string) (*models.QueueStatusResponse, error) {
				return &models.QueueStatusResponse{
					QueueLength:     3,
					AverageWaitTime: 45.0,
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *models.QueueStatusResponse) {
				if resp.QueueLength != 3 {
					t.Errorf("Expected queue length 3, got %d", resp.QueueLength)
				}
				if resp.AverageWaitTime != 45.0 {
					t.Errorf("Expected average wait time 45.0, got %f", resp.AverageWaitTime)
				}
			},
		},
		{
			name:      "Service error",
			userID:    "test-user",
			setUserID: true,
			mockGetQueueStatus: func(userID string) (*models.QueueStatusResponse, error) {
				return nil, fmt.Errorf("service error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:      "Anonymous user - userID not set in context",
			userID:    "",
			setUserID: false, // Don't set userID in context
			mockGetQueueStatus: func(userID string) (*models.QueueStatusResponse, error) {
				// Should be called with "anonymous"
				if userID != "anonymous" {
					return nil, fmt.Errorf("expected userID 'anonymous', got '%s'", userID)
				}
				return &models.QueueStatusResponse{
					QueueLength:     5,
					AverageWaitTime: 60.0,
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *models.QueueStatusResponse) {
				if resp.QueueLength != 5 {
					t.Errorf("Expected queue length 5, got %d", resp.QueueLength)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockScanService{
				getQueueStatusFunc: tt.mockGetQueueStatus,
			}
			handler := NewScanHandler(mockService, &mockLogger{})
			router := setupTestRouter()

			router.GET("/queue/status", func(c *gin.Context) {
				if tt.setUserID {
					c.Set("userID", tt.userID)
				}
				handler.GetQueueStatus(c)
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/queue/status", nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Assert response
			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var response models.QueueStatusResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				tt.checkResponse(t, &response)
			}
		})
	}
}
