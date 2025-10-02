// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package service

import (
	"fmt"
	"testing"

	"github.com/lazycatapps/trivy/backend/internal/models"
)

// BenchmarkSaveConfig measures the performance of saving a config
func BenchmarkSaveConfig(b *testing.B) {
	configService := NewConfigService(b.TempDir(), false, 4096, 1000, &mockLogger{})

	config := &models.SavedScanConfig{
		ImagePrefix: "docker.io/library/",
		Username:    "testuser",
		TLSVerify:   true,
		Severity:    []string{"HIGH", "CRITICAL"},
		Scanners:    []string{"vuln", "secret"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		configName := fmt.Sprintf("config-%d", i)
		configService.SaveConfig("user-1", configName, config)
	}
}

// BenchmarkGetConfig measures the performance of loading a config
func BenchmarkGetConfig(b *testing.B) {
	configService := NewConfigService(b.TempDir(), false, 4096, 1000, &mockLogger{})

	// Setup: Save a config
	config := &models.SavedScanConfig{
		ImagePrefix: "docker.io/library/",
		Username:    "testuser",
		TLSVerify:   true,
	}
	configService.SaveConfig("user-1", "test-config", config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		configService.GetConfig("user-1", "test-config")
	}
}

// BenchmarkListConfigs measures the performance of listing configs
func BenchmarkListConfigs(b *testing.B) {
	configService := NewConfigService(b.TempDir(), false, 4096, 1000, &mockLogger{})

	// Setup: Create 100 configs
	config := &models.SavedScanConfig{
		ImagePrefix: "docker.io/library/",
	}
	for i := 0; i < 100; i++ {
		configName := fmt.Sprintf("config-%d", i)
		configService.SaveConfig("user-1", configName, config)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		configService.ListConfigs("user-1")
	}
}

// BenchmarkDeleteConfig measures the performance of deleting a config
func BenchmarkDeleteConfig(b *testing.B) {
	configService := NewConfigService(b.TempDir(), false, 4096, 1000, &mockLogger{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Setup: Create a config
		config := &models.SavedScanConfig{ImagePrefix: "docker.io/"}
		configName := fmt.Sprintf("config-%d", i)
		configService.SaveConfig("user-1", configName, config)
		b.StartTimer()

		// Measure delete
		configService.DeleteConfig("user-1", configName)
	}
}

// BenchmarkSaveConfigLarge measures saving a large config
func BenchmarkSaveConfigLarge(b *testing.B) {
	configService := NewConfigService(b.TempDir(), false, 10240, 1000, &mockLogger{})

	// Create a large config (near max size)
	config := &models.SavedScanConfig{
		ImagePrefix: "registry.very-long-domain-name.example.com:5000/organization/project/subproject/",
		Username:    "very-long-username-for-testing-purposes",
		TLSVerify:   true,
		Severity:    []string{"UNKNOWN", "LOW", "MEDIUM", "HIGH", "CRITICAL"},
		Scanners:    []string{"vuln", "secret", "misconfig", "license"},
		PkgTypes:    []string{"os", "library"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		configName := fmt.Sprintf("large-config-%d", i)
		configService.SaveConfig("user-1", configName, config)
	}
}

// BenchmarkConcurrentConfigRead measures concurrent config reads
func BenchmarkConcurrentConfigRead(b *testing.B) {
	configService := NewConfigService(b.TempDir(), false, 4096, 1000, &mockLogger{})

	// Setup: Create configs
	for i := 0; i < 10; i++ {
		config := &models.SavedScanConfig{
			ImagePrefix: fmt.Sprintf("registry-%d.example.com/", i),
		}
		configName := fmt.Sprintf("config-%d", i)
		configService.SaveConfig("user-1", configName, config)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			configName := fmt.Sprintf("config-%d", i%10)
			configService.GetConfig("user-1", configName)
			i++
		}
	})
}

// BenchmarkConcurrentConfigWrite measures concurrent config writes
func BenchmarkConcurrentConfigWrite(b *testing.B) {
	configService := NewConfigService(b.TempDir(), false, 4096, 10000, &mockLogger{})

	config := &models.SavedScanConfig{
		ImagePrefix: "docker.io/library/",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			configName := fmt.Sprintf("config-%d-%d", b.N, i)
			configService.SaveConfig("user-1", configName, config)
			i++
		}
	})
}

// mockLogger is defined in scan_service_test.go
