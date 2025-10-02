// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockReportService implements service.ReportService for testing
type mockReportService struct {
	getReportFunc     func(taskID, format string) ([]byte, string, error)
	getReportPathFunc func(taskID, format string) (string, error)
}

func (m *mockReportService) GetReport(taskID, format string) ([]byte, string, error) {
	if m.getReportFunc != nil {
		return m.getReportFunc(taskID, format)
	}
	return nil, "", fmt.Errorf("not implemented")
}

func (m *mockReportService) GetReportPath(taskID, format string) (string, error) {
	if m.getReportPathFunc != nil {
		return m.getReportPathFunc(taskID, format)
	}
	return "", fmt.Errorf("not implemented")
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestDownloadReport tests the DownloadReport handler
func TestDownloadReport(t *testing.T) {
	tests := []struct {
		name           string
		taskID         string
		format         string
		mockGetReport  func(taskID, format string) ([]byte, string, error)
		expectedStatus int
		checkHeaders   func(*testing.T, http.Header)
		checkBody      func(*testing.T, []byte)
	}{
		{
			name:   "Download JSON report successfully",
			taskID: "task-123",
			format: "json",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return []byte(`{"test": "data"}`), "application/json", nil
			},
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				if contentType := headers.Get("Content-Type"); contentType != "application/json" {
					t.Errorf("Expected Content-Type 'application/json', got %s", contentType)
				}
				if disposition := headers.Get("Content-Disposition"); disposition == "" {
					t.Error("Expected Content-Disposition header to be set")
				}
			},
			checkBody: func(t *testing.T, body []byte) {
				expected := `{"test": "data"}`
				if string(body) != expected {
					t.Errorf("Expected body '%s', got '%s'", expected, string(body))
				}
			},
		},
		{
			name:   "Download HTML report successfully",
			taskID: "task-456",
			format: "html",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return []byte("<html></html>"), "text/html", nil
			},
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				if contentType := headers.Get("Content-Type"); contentType != "text/html" {
					t.Errorf("Expected Content-Type 'text/html', got %s", contentType)
				}
			},
		},
		{
			name:           "Invalid format",
			taskID:         "task-789",
			format:         "invalid",
			mockGetReport:  nil, // Should not be called
			expectedStatus: http.StatusBadRequest,
			checkBody: func(t *testing.T, body []byte) {
				// Response should contain error message
				bodyStr := string(body)
				if bodyStr == "" {
					t.Error("Expected error message in response body")
				}
			},
		},
		{
			name:   "Report not found",
			taskID: "non-existent",
			format: "json",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return nil, "", fmt.Errorf("task not found")
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "Scan not completed",
			taskID: "task-pending",
			format: "json",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return nil, "", fmt.Errorf("scan not completed")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "Internal server error",
			taskID: "task-error",
			format: "json",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return nil, "", fmt.Errorf("internal error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "Download CycloneDX report successfully",
			taskID: "task-cyclonedx",
			format: "cyclonedx",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return []byte(`{"bomFormat": "CycloneDX"}`), "application/vnd.cyclonedx+json", nil
			},
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				if contentType := headers.Get("Content-Type"); contentType != "application/vnd.cyclonedx+json" {
					t.Errorf("Expected Content-Type 'application/vnd.cyclonedx+json', got %s", contentType)
				}
				disposition := headers.Get("Content-Disposition")
				if disposition == "" {
					t.Error("Expected Content-Disposition header to be set")
				}
				// Check filename has .cyclonedx.json extension
				if !contains(disposition, ".cyclonedx.json") {
					t.Errorf("Expected filename to have .cyclonedx.json extension, got %s", disposition)
				}
			},
		},
		{
			name:   "Download SPDX report successfully",
			taskID: "task-spdx",
			format: "spdx",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return []byte(`{"spdxVersion": "SPDX-2.3"}`), "application/spdx+json", nil
			},
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				if contentType := headers.Get("Content-Type"); contentType != "application/spdx+json" {
					t.Errorf("Expected Content-Type 'application/spdx+json', got %s", contentType)
				}
				disposition := headers.Get("Content-Disposition")
				if disposition == "" {
					t.Error("Expected Content-Disposition header to be set")
				}
				// Check filename has .spdx.json extension
				if !contains(disposition, ".spdx.json") {
					t.Errorf("Expected filename to have .spdx.json extension, got %s", disposition)
				}
			},
		},
		{
			name:   "Download SARIF report successfully",
			taskID: "task-sarif",
			format: "sarif",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return []byte(`{"version": "2.1.0", "$schema": "https://json.schemastore.org/sarif-2.1.0.json"}`), "application/sarif+json", nil
			},
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				if contentType := headers.Get("Content-Type"); contentType != "application/sarif+json" {
					t.Errorf("Expected Content-Type 'application/sarif+json', got %s", contentType)
				}
				disposition := headers.Get("Content-Disposition")
				if disposition == "" {
					t.Error("Expected Content-Disposition header to be set")
				}
				// Check filename has .sarif extension
				if !contains(disposition, ".sarif") {
					t.Errorf("Expected filename to have .sarif extension, got %s", disposition)
				}
			},
		},
		{
			name:   "Download table report successfully",
			taskID: "task-table",
			format: "table",
			mockGetReport: func(taskID, format string) ([]byte, string, error) {
				return []byte("CVE-2023-1234  HIGH  nginx  1.21.0  1.21.6"), "text/plain", nil
			},
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				if contentType := headers.Get("Content-Type"); contentType != "text/plain" {
					t.Errorf("Expected Content-Type 'text/plain', got %s", contentType)
				}
				disposition := headers.Get("Content-Disposition")
				if disposition == "" {
					t.Error("Expected Content-Disposition header to be set")
				}
				// Check filename has .txt extension
				if !contains(disposition, ".txt") {
					t.Errorf("Expected filename to have .txt extension, got %s", disposition)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockReportService{
				getReportFunc: tt.mockGetReport,
			}
			handler := NewReportHandler(mockService, &mockLogger{})
			router := setupTestRouter()

			router.GET("/scan/:id/report/:format", handler.DownloadReport)

			// Create request
			url := fmt.Sprintf("/scan/%s/report/%s", tt.taskID, tt.format)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check headers
			if tt.checkHeaders != nil && w.Code == http.StatusOK {
				tt.checkHeaders(t, w.Header())
			}

			// Check body
			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.Bytes())
			}
		})
	}
}
