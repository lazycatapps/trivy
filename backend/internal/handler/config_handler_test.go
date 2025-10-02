// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lazycatapps/trivy/backend/internal/models"
	"github.com/lazycatapps/trivy/backend/internal/service"
)

// TestListConfigs tests the ListConfigs handler
func TestListConfigs(t *testing.T) {
	// getUserIdentifier in config_handler.go returns session.Email + "_" + session.UserID
	const testUserID = "user-123"
	const testEmail = "test@example.com"
	const testUserIdentifier = testEmail + "_" + testUserID

	tests := []struct {
		name           string
		setupSession   func(*gin.Context)
		setupConfigs   func(*service.ConfigService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "List configs successfully",
			setupSession: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: testUserID,
					Email:  testEmail,
				})
			},
			setupConfigs: func(cs *service.ConfigService) {
				// Save some test configs - use the same identifier as getUserIdentifier()
				if err := cs.SaveConfig(testUserIdentifier, "config1", &models.SavedScanConfig{
					ImagePrefix: "docker.io/",
				}); err != nil {
					t.Fatalf("Failed to save config1: %v", err)
				}
				if err := cs.SaveConfig(testUserIdentifier, "config2", &models.SavedScanConfig{
					ImagePrefix: "registry.example.com/",
				}); err != nil {
					t.Fatalf("Failed to save config2: %v", err)
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				configs, ok := resp["configs"].([]interface{})
				if !ok {
					t.Error("Expected configs array in response")
					return
				}
				if len(configs) < 2 {
					t.Errorf("Expected at least 2 configs, got %d", len(configs))
				}
			},
		},
		{
			name: "List configs without session",
			setupSession: func(c *gin.Context) {
				// No session
			},
			setupConfigs:   func(cs *service.ConfigService) {},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			configService := service.NewConfigService(t.TempDir(), true, 4096, 1000, &mockLogger{})
			if tt.setupConfigs != nil {
				tt.setupConfigs(configService)
			}

			handler := NewConfigHandler(configService, false, &mockLogger{})
			router := setupTestRouter()

			router.GET("/configs", func(c *gin.Context) {
				tt.setupSession(c)
				handler.ListConfigs(c)
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/configs", nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// Assert response
			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				tt.checkResponse(t, response)
			}
		})
	}
}

// TestGetConfig tests the GetConfig handler
func TestGetConfig(t *testing.T) {
	const testUserID = "user-123"
	const testEmail = "test@example.com"
	const testUserIdentifier = testEmail + "_" + testUserID

	tests := []struct {
		name           string
		configName     string
		setupSession   func(*gin.Context)
		setupConfigs   func(*service.ConfigService)
		expectedStatus int
		checkResponse  func(*testing.T, *models.SavedScanConfig)
	}{
		{
			name:       "Get config successfully",
			configName: "test-config",
			setupSession: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: testUserID,
					Email:  testEmail,
				})
			},
			setupConfigs: func(cs *service.ConfigService) {
				if err := cs.SaveConfig(testUserIdentifier, "test-config", &models.SavedScanConfig{
					ImagePrefix: "docker.io/",
					TLSVerify:   true,
					Severity:    []string{"HIGH", "CRITICAL"},
				}); err != nil {
					t.Fatalf("Failed to save test-config: %v", err)
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, config *models.SavedScanConfig) {
				if config.ImagePrefix != "docker.io/" {
					t.Errorf("Expected image prefix 'docker.io/', got %s", config.ImagePrefix)
				}
			},
		},
		{
			name:       "Config not found",
			configName: "non-existent",
			setupSession: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: testUserID,
					Email:  testEmail,
				})
			},
			setupConfigs:   func(cs *service.ConfigService) {},
			expectedStatus: http.StatusOK, // Returns empty config, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			configService := service.NewConfigService(t.TempDir(), true, 4096, 1000, &mockLogger{})
			if tt.setupConfigs != nil {
				tt.setupConfigs(configService)
			}

			handler := NewConfigHandler(configService, false, &mockLogger{})
			router := setupTestRouter()

			router.GET("/config/:name", func(c *gin.Context) {
				tt.setupSession(c)
				handler.GetConfig(c)
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/config/"+tt.configName, nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Assert response
			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var config models.SavedScanConfig
				if err := json.Unmarshal(w.Body.Bytes(), &config); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				tt.checkResponse(t, &config)
			}
		})
	}
}

// TestSaveConfig tests the SaveConfig handler
func TestSaveConfig(t *testing.T) {
	const testUserID = "user-123"
	const testEmail = "test@example.com"

	tests := []struct {
		name           string
		configName     string
		requestBody    interface{}
		setupSession   func(*gin.Context)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:       "Save config successfully",
			configName: "test-config",
			requestBody: map[string]interface{}{
				"imagePrefix": "docker.io/",
				"tlsVerify":   true,
				"severity":    []string{"HIGH", "CRITICAL"},
			},
			setupSession: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: testUserID,
					Email:  testEmail,
				})
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["message"] != "Configuration saved successfully" {
					t.Errorf("Expected success message, got %v", resp["message"])
				}
			},
		},
		{
			name:        "Invalid JSON body",
			configName:  "test-config",
			requestBody: "invalid json",
			setupSession: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: testUserID,
					Email:  testEmail,
				})
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			configService := service.NewConfigService(t.TempDir(), true, 4096, 1000, &mockLogger{})
			handler := NewConfigHandler(configService, false, &mockLogger{})
			router := setupTestRouter()

			router.POST("/config/:name", func(c *gin.Context) {
				tt.setupSession(c)
				handler.SaveConfig(c)
			})

			// Create request
			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, _ = json.Marshal(tt.requestBody)
			}

			req := httptest.NewRequest(http.MethodPost, "/config/"+tt.configName, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// Assert response
			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				tt.checkResponse(t, response)
			}
		})
	}
}

// TestDeleteConfig tests the DeleteConfig handler
func TestDeleteConfig(t *testing.T) {
	const testUserID = "user-123"
	const testEmail = "test@example.com"
	const testUserIdentifier = testEmail + "_" + testUserID

	tests := []struct {
		name           string
		configName     string
		setupSession   func(*gin.Context)
		setupConfigs   func(*service.ConfigService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:       "Delete config successfully",
			configName: "test-config",
			setupSession: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: testUserID,
					Email:  testEmail,
				})
			},
			setupConfigs: func(cs *service.ConfigService) {
				if err := cs.SaveConfig(testUserIdentifier, "test-config", &models.SavedScanConfig{
					ImagePrefix: "docker.io/",
				}); err != nil {
					t.Fatalf("Failed to save test-config: %v", err)
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["message"] != "Configuration deleted successfully" {
					t.Errorf("Expected success message, got %v", resp["message"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			configService := service.NewConfigService(t.TempDir(), true, 4096, 1000, &mockLogger{})
			if tt.setupConfigs != nil {
				tt.setupConfigs(configService)
			}

			handler := NewConfigHandler(configService, false, &mockLogger{})
			router := setupTestRouter()

			router.DELETE("/config/:name", func(c *gin.Context) {
				tt.setupSession(c)
				handler.DeleteConfig(c)
			})

			// Create request
			req := httptest.NewRequest(http.MethodDelete, "/config/"+tt.configName, nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Assert response
			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				tt.checkResponse(t, response)
			}
		})
	}
}

// TestGetLastUsedConfig tests the GetLastUsedConfig handler
func TestGetLastUsedConfig(t *testing.T) {
	const testUserID = "user-123"
	const testEmail = "test@example.com"
	const testUserIdentifier = testEmail + "_" + testUserID

	tests := []struct {
		name           string
		setupSession   func(*gin.Context)
		setupConfigs   func(*service.ConfigService)
		expectedStatus int
		expectedName   string
	}{
		{
			name: "Get last used config successfully",
			setupSession: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: testUserID,
					Email:  testEmail,
				})
			},
			setupConfigs: func(cs *service.ConfigService) {
				// Save a config and mark it as last used
				if err := cs.SaveConfig(testUserIdentifier, "my-config", &models.SavedScanConfig{
					ImagePrefix: "docker.io/",
				}); err != nil {
					t.Fatalf("Failed to save config: %v", err)
				}
				// Mark as last used by calling SetLastUsedConfig
				if err := cs.SetLastUsedConfig(testUserIdentifier, "my-config"); err != nil {
					t.Fatalf("Failed to set last used config: %v", err)
				}
			},
			expectedStatus: http.StatusOK,
			expectedName:   "my-config",
		},
		{
			name: "No last used config",
			setupSession: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: testUserID,
					Email:  testEmail,
				})
			},
			setupConfigs:   func(cs *service.ConfigService) {},
			expectedStatus: http.StatusOK,
			expectedName:   "", // Empty string when no last used config
		},
		{
			name: "Without session",
			setupSession: func(c *gin.Context) {
				// No session
			},
			setupConfigs:   func(cs *service.ConfigService) {},
			expectedStatus: http.StatusOK,
			expectedName:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			configService := service.NewConfigService(t.TempDir(), true, 4096, 1000, &mockLogger{})
			if tt.setupConfigs != nil {
				tt.setupConfigs(configService)
			}

			handler := NewConfigHandler(configService, false, &mockLogger{})
			router := setupTestRouter()

			router.GET("/config/last-used", func(c *gin.Context) {
				tt.setupSession(c)
				handler.GetLastUsedConfig(c)
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/config/last-used", nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// Assert response
			if w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				name := ""
				if n, ok := response["name"].(string); ok {
					name = n
				}
				if name != tt.expectedName {
					t.Errorf("Expected name '%s', got '%s'", tt.expectedName, name)
				}
			}
		})
	}
}

// TestGetUserIdentifier tests the getUserIdentifier function
func TestGetUserIdentifier(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func(*gin.Context)
		expectedResult string
	}{
		{
			name: "Valid session with email and userID",
			setupContext: func(c *gin.Context) {
				c.Set("session", &service.SessionInfo{
					UserID: "user-123",
					Email:  "test@example.com",
				})
			},
			expectedResult: "test@example.com_user-123",
		},
		{
			name: "Session not found in context",
			setupContext: func(c *gin.Context) {
				// No session set
			},
			expectedResult: "anonymous",
		},
		{
			name: "Session has wrong type",
			setupContext: func(c *gin.Context) {
				c.Set("session", "not-a-session-info")
			},
			expectedResult: "anonymous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			router := setupTestRouter()
			var result string

			router.GET("/test", func(c *gin.Context) {
				tt.setupContext(c)
				result = getUserIdentifier(c)
				c.JSON(http.StatusOK, gin.H{"result": result})
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert result
			if result != tt.expectedResult {
				t.Errorf("Expected getUserIdentifier to return '%s', got '%s'", tt.expectedResult, result)
			}
		})
	}
}
