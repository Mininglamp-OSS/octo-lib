package util

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Private IPv4 ranges
		{"10.0.0.0/8 - start", "10.0.0.1", true},
		{"10.0.0.0/8 - middle", "10.128.0.1", true},
		{"10.0.0.0/8 - end", "10.255.255.254", true},
		{"172.16.0.0/12 - start", "172.16.0.1", true},
		{"172.16.0.0/12 - end", "172.31.255.254", true},
		{"192.168.0.0/16 - start", "192.168.0.1", true},
		{"192.168.0.0/16 - end", "192.168.255.254", true},
		{"127.0.0.0/8 - localhost", "127.0.0.1", true},
		{"127.0.0.0/8 - other", "127.0.0.2", true},
		{"169.254.0.0/16 - link local", "169.254.1.1", true},

		// Public IPv4
		{"public IP - Google DNS", "8.8.8.8", false},
		{"public IP - Cloudflare", "1.1.1.1", false},
		{"public IP - random", "203.0.113.1", false},
		{"172.15.x.x - not private", "172.15.0.1", false},
		{"172.32.x.x - not private", "172.32.0.1", false},

		// Private IPv6 ranges
		{"::1 - loopback", "::1", true},
		{"fc00::/7 - unique local", "fc00::1", true},
		{"fd00::/8 - unique local", "fd00::1", true},
		{"fe80::/10 - link local", "fe80::1", true},

		// Public IPv6
		{"public IPv6", "2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestValidateExternalURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Valid external URLs
		{"valid https", "https://example.com", false},
		{"valid http", "http://example.com", false},
		{"valid with path", "https://example.com/path", false},
		{"valid with port", "https://example.com:8080/path", false},

		// Invalid schemes
		{"file scheme", "file:///etc/passwd", true},
		{"ftp scheme", "ftp://ftp.example.com", true},
		{"javascript scheme", "javascript:alert(1)", true},

		// Invalid URLs
		{"empty string", "", true},
		{"malformed URL", "://invalid", true},
		{"empty host", "https:///path", true},

		// Private IPs (should be rejected)
		{"private IP 10.x", "http://10.0.0.1", true},
		{"private IP 192.168.x", "http://192.168.1.1", true},
		{"private IP 172.16.x", "http://172.16.0.1", true},
		{"localhost", "http://127.0.0.1", true},
		{"localhost IPv6", "http://[::1]", true},

		// Public IPs (should be allowed)
		{"public IP", "http://8.8.8.8", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExternalURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExternalURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestPrivateNetworksInitialized(t *testing.T) {
	// Verify that privateNetworks is properly initialized
	if len(privateNetworks) == 0 {
		t.Error("privateNetworks should be initialized with CIDR ranges")
	}

	expectedCount := 8 // Number of private ranges defined
	if len(privateNetworks) != expectedCount {
		t.Errorf("privateNetworks has %d entries, want %d", len(privateNetworks), expectedCount)
	}
}

func BenchmarkIsPrivateIP(b *testing.B) {
	ip := net.ParseIP("192.168.1.100")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isPrivateIP(ip)
	}
}

func BenchmarkIsPrivateIP_PublicIP(b *testing.B) {
	ip := net.ParseIP("8.8.8.8")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isPrivateIP(ip)
	}
}
