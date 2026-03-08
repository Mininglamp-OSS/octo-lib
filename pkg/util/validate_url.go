package util

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateNetworks holds pre-parsed CIDR networks for private IP ranges.
// Parsing once at init time avoids repeated parsing on every isPrivateIP call.
var privateNetworks []*net.IPNet

func init() {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// This should never happen with hardcoded valid CIDRs
			panic(fmt.Sprintf("invalid CIDR %q: %v", cidr, err))
		}
		privateNetworks = append(privateNetworks, network)
	}
}

// ValidateExternalURL validates that a URL is safe to request (prevents SSRF).
// It rejects private/loopback IPs and non-http(s) schemes.
func ValidateExternalURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}

	// Direct IP check
	ip := net.ParseIP(host)
	if ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("private IP not allowed: %s", ip)
		}
		return nil
	}

	// Resolve hostname and check resolved IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS resolution failed for %s: %w", host, err)
	}

	for _, resolved := range ips {
		if isPrivateIP(resolved) {
			return fmt.Errorf("host %s resolves to private IP %s", host, resolved)
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	for _, network := range privateNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
