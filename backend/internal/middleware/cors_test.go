// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                  string
		allowedOrigins        []string
		requestOrigin         string
		requestMethod         string
		expectedOrigin        string
		expectedMethods       string
		expectedHeaders       string
		expectedCredentials   string
		expectedStatus        int
		shouldHaveCORSHeaders bool
	}{
		{
			name:                  "Wildcard allows all origins with Origin header",
			allowedOrigins:        []string{"*"},
			requestOrigin:         "https://example.com",
			requestMethod:         "GET",
			expectedOrigin:        "https://example.com",
			expectedMethods:       "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders:       "Content-Type, Authorization",
			expectedCredentials:   "true",
			expectedStatus:        http.StatusOK,
			shouldHaveCORSHeaders: true,
		},
		{
			name:                  "Wildcard without Origin header",
			allowedOrigins:        []string{"*"},
			requestOrigin:         "",
			requestMethod:         "GET",
			expectedOrigin:        "*",
			expectedMethods:       "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders:       "Content-Type, Authorization",
			expectedCredentials:   "",
			expectedStatus:        http.StatusOK,
			shouldHaveCORSHeaders: true,
		},
		{
			name:                  "Exact origin match",
			allowedOrigins:        []string{"https://app.example.com"},
			requestOrigin:         "https://app.example.com",
			requestMethod:         "POST",
			expectedOrigin:        "https://app.example.com",
			expectedMethods:       "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders:       "Content-Type, Authorization",
			expectedCredentials:   "true",
			expectedStatus:        http.StatusOK,
			shouldHaveCORSHeaders: true,
		},
		{
			name:                  "Origin not in allowed list",
			allowedOrigins:        []string{"https://app.example.com"},
			requestOrigin:         "https://evil.com",
			requestMethod:         "GET",
			expectedOrigin:        "",
			expectedMethods:       "",
			expectedHeaders:       "",
			expectedCredentials:   "",
			expectedStatus:        http.StatusOK,
			shouldHaveCORSHeaders: false,
		},
		{
			name:                  "Multiple allowed origins - match first",
			allowedOrigins:        []string{"https://app1.com", "https://app2.com"},
			requestOrigin:         "https://app1.com",
			requestMethod:         "GET",
			expectedOrigin:        "https://app1.com",
			expectedMethods:       "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders:       "Content-Type, Authorization",
			expectedCredentials:   "true",
			expectedStatus:        http.StatusOK,
			shouldHaveCORSHeaders: true,
		},
		{
			name:                  "Multiple allowed origins - match second",
			allowedOrigins:        []string{"https://app1.com", "https://app2.com"},
			requestOrigin:         "https://app2.com",
			requestMethod:         "GET",
			expectedOrigin:        "https://app2.com",
			expectedMethods:       "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders:       "Content-Type, Authorization",
			expectedCredentials:   "true",
			expectedStatus:        http.StatusOK,
			shouldHaveCORSHeaders: true,
		},
		{
			name:                  "OPTIONS preflight request with wildcard",
			allowedOrigins:        []string{"*"},
			requestOrigin:         "https://example.com",
			requestMethod:         "OPTIONS",
			expectedOrigin:        "https://example.com",
			expectedMethods:       "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders:       "Content-Type, Authorization",
			expectedCredentials:   "true",
			expectedStatus:        http.StatusNoContent,
			shouldHaveCORSHeaders: true,
		},
		{
			name:                  "OPTIONS preflight request with exact origin",
			allowedOrigins:        []string{"https://app.example.com"},
			requestOrigin:         "https://app.example.com",
			requestMethod:         "OPTIONS",
			expectedOrigin:        "https://app.example.com",
			expectedMethods:       "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders:       "Content-Type, Authorization",
			expectedCredentials:   "true",
			expectedStatus:        http.StatusNoContent,
			shouldHaveCORSHeaders: true,
		},
		{
			name:                  "OPTIONS preflight request - origin not allowed",
			allowedOrigins:        []string{"https://app.example.com"},
			requestOrigin:         "https://evil.com",
			requestMethod:         "OPTIONS",
			expectedOrigin:        "",
			expectedMethods:       "",
			expectedHeaders:       "",
			expectedCredentials:   "",
			expectedStatus:        http.StatusNoContent,
			shouldHaveCORSHeaders: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Gin router with CORS middleware
			router := gin.New()
			router.Use(CORS(tt.allowedOrigins))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})
			router.POST("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			// Create request
			req := httptest.NewRequest(tt.requestMethod, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}

			// Record response
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check CORS headers
			if tt.shouldHaveCORSHeaders {
				origin := w.Header().Get("Access-Control-Allow-Origin")
				if origin != tt.expectedOrigin {
					t.Errorf("Expected Access-Control-Allow-Origin '%s', got '%s'", tt.expectedOrigin, origin)
				}

				methods := w.Header().Get("Access-Control-Allow-Methods")
				if methods != tt.expectedMethods {
					t.Errorf("Expected Access-Control-Allow-Methods '%s', got '%s'", tt.expectedMethods, methods)
				}

				headers := w.Header().Get("Access-Control-Allow-Headers")
				if headers != tt.expectedHeaders {
					t.Errorf("Expected Access-Control-Allow-Headers '%s', got '%s'", tt.expectedHeaders, headers)
				}

				credentials := w.Header().Get("Access-Control-Allow-Credentials")
				if credentials != tt.expectedCredentials {
					t.Errorf("Expected Access-Control-Allow-Credentials '%s', got '%s'", tt.expectedCredentials, credentials)
				}
			} else {
				// Should not have CORS headers
				if w.Header().Get("Access-Control-Allow-Origin") != "" {
					t.Error("Should not have Access-Control-Allow-Origin header")
				}
				if w.Header().Get("Access-Control-Allow-Methods") != "" {
					t.Error("Should not have Access-Control-Allow-Methods header")
				}
			}
		})
	}
}

func TestCORSWithOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		originsCSV     string
		requestOrigin  string
		expectedOrigin string
		shouldAllow    bool
	}{
		{
			name:           "Empty string defaults to wildcard",
			originsCSV:     "",
			requestOrigin:  "https://example.com",
			expectedOrigin: "https://example.com",
			shouldAllow:    true,
		},
		{
			name:           "Whitespace only defaults to wildcard",
			originsCSV:     "   ",
			requestOrigin:  "https://example.com",
			expectedOrigin: "https://example.com",
			shouldAllow:    true,
		},
		{
			name:           "Single origin",
			originsCSV:     "https://app.example.com",
			requestOrigin:  "https://app.example.com",
			expectedOrigin: "https://app.example.com",
			shouldAllow:    true,
		},
		{
			name:           "Multiple origins - comma separated",
			originsCSV:     "https://app1.com,https://app2.com",
			requestOrigin:  "https://app2.com",
			expectedOrigin: "https://app2.com",
			shouldAllow:    true,
		},
		{
			name:           "Multiple origins with spaces",
			originsCSV:     "https://app1.com, https://app2.com , https://app3.com",
			requestOrigin:  "https://app3.com",
			expectedOrigin: "https://app3.com",
			shouldAllow:    true,
		},
		{
			name:           "Origin not in list",
			originsCSV:     "https://app1.com,https://app2.com",
			requestOrigin:  "https://evil.com",
			expectedOrigin: "",
			shouldAllow:    false,
		},
		{
			name:           "Wildcard in CSV",
			originsCSV:     "*",
			requestOrigin:  "https://any-origin.com",
			expectedOrigin: "https://any-origin.com",
			shouldAllow:    true,
		},
		{
			name:           "Empty values in CSV ignored",
			originsCSV:     "https://app1.com,,https://app2.com",
			requestOrigin:  "https://app2.com",
			expectedOrigin: "https://app2.com",
			shouldAllow:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(CORSWithOrigins(tt.originsCSV))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			origin := w.Header().Get("Access-Control-Allow-Origin")
			if tt.shouldAllow {
				if origin != tt.expectedOrigin {
					t.Errorf("Expected Access-Control-Allow-Origin '%s', got '%s'", tt.expectedOrigin, origin)
				}
			} else {
				if origin != "" {
					t.Errorf("Expected no Access-Control-Allow-Origin header, got '%s'", origin)
				}
			}
		})
	}
}
