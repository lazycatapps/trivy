// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

// Package models defines data structures for the Trivy Web UI application.
package models

import (
	"sync"
	"time"
)

// ScanStatus represents the current state of a scan task.
type ScanStatus string

const (
	ScanStatusQueued    ScanStatus = "queued"    // Task in queue, waiting to execute
	ScanStatusRunning   ScanStatus = "running"   // Task is currently executing
	ScanStatusCompleted ScanStatus = "completed" // Task completed successfully
	ScanStatusFailed    ScanStatus = "failed"    // Task failed with error
)

// ScanTask represents a Trivy security scan task.
// It tracks task metadata, status, logs, scan results, and provides real-time log streaming.
type ScanTask struct {
	ID        string     `json:"id"`                // Unique task identifier (UUID)
	UserID    string     `json:"userId"`            // User ID (for OIDC multi-tenancy)
	Image     string     `json:"image"`             // Container image to scan
	Status    ScanStatus `json:"status"`            // Current task status
	Message   string     `json:"message"`           // Human-readable status message
	StartTime time.Time  `json:"startTime"`         // Task creation timestamp
	EndTime   *time.Time `json:"endTime,omitempty"` // Task completion timestamp

	// Scan configuration
	ScanConfig *ScanConfig `json:"scanConfig,omitempty"` // Scan parameters

	// Queue information
	QueuePosition int `json:"queuePosition,omitempty"` // Position in queue (0 = running)

	// Scan results
	Result       *ScanResult   `json:"result,omitempty"`       // Parsed scan results
	Output       string        `json:"output"`                 // Complete log output
	ErrorOutput  string        `json:"errorOutput,omitempty"`  // Error message (if failed)
	TrivyVersion *TrivyVersion `json:"trivyVersion,omitempty"` // Trivy Server version info at scan time

	// Log streaming
	LogLines     []string      `json:"-"` // In-memory log lines (not serialized)
	LogListeners []chan string `json:"-"` // Active log stream subscribers (SSE)
	logMu        sync.Mutex    // Mutex for thread-safe log operations
}

// ScanConfig represents scan configuration parameters.
type ScanConfig struct {
	Username          string   `json:"username,omitempty"`          // Registry username
	Password          string   `json:"password,omitempty"`          // Registry password (masked in logs)
	TLSVerify         bool     `json:"tlsVerify"`                   // Enable TLS certificate verification
	Severity          []string `json:"severity,omitempty"`          // Vulnerability severity filter
	IgnoreUnfixed     bool     `json:"ignoreUnfixed"`               // Ignore unfixed vulnerabilities
	Scanners          []string `json:"scanners,omitempty"`          // Scanner types (vuln, misconfig, secret, license)
	DetectionPriority string   `json:"detectionPriority,omitempty"` // Detection priority (precise, comprehensive)
	PkgTypes          []string `json:"pkgTypes,omitempty"`          // Package types (os, library)
	Format            string   `json:"format,omitempty"`            // Output format (json, table, sarif, etc.)
}

// ScanResult represents parsed scan results from Trivy JSON output.
type ScanResult struct {
	Format  string                `json:"format"`            // Output format (json, table, etc.)
	Data    string                `json:"data"`              // Raw Trivy output
	Summary *VulnerabilitySummary `json:"summary,omitempty"` // Vulnerability statistics (JSON format only)
}

// VulnerabilitySummary represents aggregated vulnerability statistics.
type VulnerabilitySummary struct {
	Total    int `json:"total"`    // Total number of vulnerabilities
	Critical int `json:"critical"` // Number of CRITICAL vulnerabilities
	High     int `json:"high"`     // Number of HIGH vulnerabilities
	Medium   int `json:"medium"`   // Number of MEDIUM vulnerabilities
	Low      int `json:"low"`      // Number of LOW vulnerabilities
	Unknown  int `json:"unknown"`  // Number of UNKNOWN severity vulnerabilities
}

// NewScanTask creates a new scan task with initial queued status.
func NewScanTask(id, userID, image string, config *ScanConfig) *ScanTask {
	return &ScanTask{
		ID:           id,
		UserID:       userID,
		Image:        image,
		Status:       ScanStatusQueued,
		Message:      "Task created and queued",
		StartTime:    time.Now(),
		ScanConfig:   config,
		LogLines:     []string{},
		LogListeners: []chan string{},
	}
}

// AddLog appends a log line to the task and broadcasts it to all active listeners.
// Thread-safe for concurrent access.
func (t *ScanTask) AddLog(line string) {
	t.logMu.Lock()
	defer t.logMu.Unlock()

	t.LogLines = append(t.LogLines, line)
	// Broadcast to all SSE listeners
	for _, ch := range t.LogListeners {
		select {
		case ch <- line:
			// Successfully sent
		default:
			// Channel is full or closed, skip this listener
		}
	}
}

// AddLogListener creates a new log listener channel for SSE streaming.
// Returns a buffered channel (100 messages) that will receive new log lines.
func (t *ScanTask) AddLogListener() chan string {
	t.logMu.Lock()
	defer t.logMu.Unlock()

	ch := make(chan string, 100)
	t.LogListeners = append(t.LogListeners, ch)
	return ch
}

// RemoveLogListener removes and closes a log listener channel.
// Should be called when an SSE client disconnects.
func (t *ScanTask) RemoveLogListener(ch chan string) {
	t.logMu.Lock()
	defer t.logMu.Unlock()

	for i, listener := range t.LogListeners {
		if listener == ch {
			t.LogListeners = append(t.LogListeners[:i], t.LogListeners[i+1:]...)
			close(ch)
			break
		}
	}
}

// CloseAllLogListeners closes all active log listener channels.
// Called when task completes to notify all SSE clients.
func (t *ScanTask) CloseAllLogListeners() {
	t.logMu.Lock()
	defer t.logMu.Unlock()

	for _, ch := range t.LogListeners {
		close(ch)
	}
	t.LogListeners = []chan string{}
}

// GetLogLines returns a copy of all log lines.
// Thread-safe for concurrent access.
func (t *ScanTask) GetLogLines() []string {
	t.logMu.Lock()
	defer t.logMu.Unlock()

	logs := make([]string, len(t.LogLines))
	copy(logs, t.LogLines)
	return logs
}

// ToSummary converts a ScanTask to TaskSummary (for list queries).
func (t *ScanTask) ToSummary() *TaskSummary {
	summary := &TaskSummary{
		ID:            t.ID,
		Image:         t.Image,
		Status:        string(t.Status),
		Message:       t.Message,
		StartTime:     t.StartTime,
		EndTime:       t.EndTime,
		QueuePosition: t.QueuePosition,
	}

	// Only include summary if result exists
	if t.Result != nil {
		summary.Summary = t.Result.Summary
	}

	return summary
}

// ScanRequest represents the request body for creating a scan task.
type ScanRequest struct {
	Image             string   `json:"image" binding:"required"` // Container image to scan (required)
	Username          string   `json:"username"`                 // Registry username (optional)
	Password          string   `json:"password"`                 // Registry password (optional)
	TLSVerify         *bool    `json:"tlsVerify"`                // TLS verification (optional, default: true)
	Severity          []string `json:"severity"`                 // Vulnerability severity filter (optional)
	IgnoreUnfixed     bool     `json:"ignoreUnfixed"`            // Ignore unfixed vulnerabilities (optional)
	Scanners          []string `json:"scanners"`                 // Scanner types (optional, default: ["vuln"])
	DetectionPriority string   `json:"detectionPriority"`        // Detection priority (optional, default: "precise")
	PkgTypes          []string `json:"pkgTypes"`                 // Package types (optional)
	Format            string   `json:"format"`                   // Output format (optional, default: "json")
}

// TaskSummary represents a summarized view of a scan task (for list queries).
type TaskSummary struct {
	ID            string                `json:"id"`
	Image         string                `json:"image"`
	Status        string                `json:"status"`
	Message       string                `json:"message"`
	StartTime     time.Time             `json:"startTime"`
	EndTime       *time.Time            `json:"endTime,omitempty"`
	QueuePosition int                   `json:"queuePosition,omitempty"` // Position in queue (only for queued tasks)
	Summary       *VulnerabilitySummary `json:"summary,omitempty"`       // Vulnerability statistics
}

// TaskListRequest represents query parameters for listing scan tasks.
type TaskListRequest struct {
	Page      int    `form:"page,default=1"`           // Page number (default: 1)
	PageSize  int    `form:"pageSize,default=20"`      // Items per page (default: 20, max: 100)
	Status    string `form:"status"`                   // Filter by status (optional)
	SortBy    string `form:"sortBy,default=startTime"` // Sort field (default: startTime)
	SortOrder string `form:"sortOrder,default=desc"`   // Sort order: asc/desc (default: desc)
}

// TaskListResponse represents the response for task list queries.
type TaskListResponse struct {
	Total    int            `json:"total"`    // Total number of tasks matching filter
	Page     int            `json:"page"`     // Current page number
	PageSize int            `json:"pageSize"` // Items per page
	Tasks    []*TaskSummary `json:"tasks"`    // Task summaries for current page
}

// QueueStatusResponse represents the current queue status.
type QueueStatusResponse struct {
	QueueLength     int     `json:"queueLength"`     // Number of tasks waiting in queue
	AverageWaitTime float64 `json:"averageWaitTime"` // Average wait time in seconds
}
