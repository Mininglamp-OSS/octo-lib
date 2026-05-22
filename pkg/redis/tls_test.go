package redis

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	rd "github.com/go-redis/redis"
)

// writeSelfSignedCA generates a fresh self-signed CA cert PEM and writes it to a
// temp file, returning the file path.
func writeSelfSignedCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("pem encode: %v", err)
	}
	return path
}

func TestBuildTLSConfig_NoCA(t *testing.T) {
	cfg, err := BuildTLSConfig(false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.RootCAs != nil {
		t.Errorf("expected RootCAs nil when no CA file, got non-nil")
	}
	if cfg.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify=false")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want TLS 1.2 (%x)", cfg.MinVersion, tls.VersionTLS12)
	}
}

func TestBuildTLSConfig_InsecureSkipVerify(t *testing.T) {
	cfg, err := BuildTLSConfig(true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify=true")
	}
}

func TestBuildTLSConfig_CAFileMissing(t *testing.T) {
	_, err := BuildTLSConfig(false, "/nonexistent/path/ca.pem")
	if err == nil {
		t.Fatal("expected error for missing CA file, got nil")
	}
}

func TestBuildTLSConfig_CAFileInvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(path, []byte("not a pem"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := BuildTLSConfig(false, path)
	if err == nil {
		t.Fatal("expected error for invalid PEM, got nil")
	}
}

func TestBuildTLSConfig_CAFileValid(t *testing.T) {
	path := writeSelfSignedCA(t)
	cfg, err := BuildTLSConfig(false, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Errorf("expected RootCAs to be populated")
	}
}

func TestNewWithOptions_DefaultMaxRetries(t *testing.T) {
	conn := NewWithOptions(&rd.Options{Addr: "127.0.0.1:1"})
	if conn == nil || conn.client == nil {
		t.Fatal("expected non-nil conn and client")
	}
	if got := conn.client.Options().MaxRetries; got != 3 {
		t.Errorf("MaxRetries = %d, want 3 (default)", got)
	}
}

func TestNewWithOptions_RespectsCallerMaxRetries(t *testing.T) {
	conn := NewWithOptions(&rd.Options{Addr: "127.0.0.1:1", MaxRetries: 7})
	if got := conn.client.Options().MaxRetries; got != 7 {
		t.Errorf("MaxRetries = %d, want 7 (caller supplied)", got)
	}
}

func TestNewWithOptions_NilOpts(t *testing.T) {
	conn := NewWithOptions(nil)
	if conn == nil || conn.client == nil {
		t.Fatal("expected non-nil conn and client even with nil opts")
	}
	if got := conn.Options().MaxRetries; got != 3 {
		t.Errorf("MaxRetries = %d, want 3 (default applied on nil opts)", got)
	}
}

// Regression: NewWithOptions must not mutate the caller's *rd.Options.
func TestNewWithOptions_DoesNotMutateCaller(t *testing.T) {
	caller := &rd.Options{Addr: "127.0.0.1:1"} // MaxRetries left at zero
	_ = NewWithOptions(caller)
	if caller.MaxRetries != 0 {
		t.Errorf("caller's MaxRetries was mutated to %d, want 0", caller.MaxRetries)
	}
}
