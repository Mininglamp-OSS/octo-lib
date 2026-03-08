package util

import (
	"testing"

	"gopkg.in/go-playground/assert.v1"
)

func TestMD5(t *testing.T) {
	// Test basic MD5 hash
	result := MD5("hello")
	assert.Equal(t, "5d41402abc4b2a76b9719d911017c592", result)

	// Test empty string
	result = MD5("")
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", result)
}

func TestSHA1(t *testing.T) {
	// Test basic SHA1 hash
	result := SHA1("hello")
	assert.Equal(t, "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d", result)

	// Test empty string
	result = SHA1("")
	assert.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", result)
}

func TestHMACSHA1(t *testing.T) {
	// Test HMAC-SHA1
	result := HMACSHA1("secret", "hello")
	assert.Equal(t, "URIFXAX5RPhXVe/FzYlw4ZTp9Fs=", result)
}
