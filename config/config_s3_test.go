package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestS3Defaults(t *testing.T) {
	cfg := New()
	vp := viper.New()
	cfg.ConfigureWithViper(vp)

	if cfg.S3.Endpoint != "" {
		t.Errorf("S3.Endpoint default should be empty, got %q", cfg.S3.Endpoint)
	}
	if cfg.S3.Region != "" {
		t.Errorf("S3.Region default should be empty, got %q", cfg.S3.Region)
	}
	if cfg.S3.Bucket != "" {
		t.Errorf("S3.Bucket default should be empty, got %q", cfg.S3.Bucket)
	}
	if cfg.S3.UsePathStyle {
		t.Errorf("S3.UsePathStyle default should be false, got true")
	}
}

func TestS3FromViper(t *testing.T) {
	const yaml = `
s3:
  endpoint: s3.us-west-2.amazonaws.com
  region: us-west-2
  bucket: my-bucket
  accessKeyID: AKIAEXAMPLE
  secretAccessKey: secret
  sessionToken: FwoGZXIvYXdzEXAMPLE
  downloadURL: https://d123.cloudfront.net
  prefix: prod/
  usePathStyle: true
`
	vp := viper.New()
	vp.SetConfigType("yaml")
	if err := vp.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	cfg := New()
	cfg.ConfigureWithViper(vp)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Endpoint", cfg.S3.Endpoint, "s3.us-west-2.amazonaws.com"},
		{"Region", cfg.S3.Region, "us-west-2"},
		{"Bucket", cfg.S3.Bucket, "my-bucket"},
		{"AccessKeyID", cfg.S3.AccessKeyID, "AKIAEXAMPLE"},
		{"SecretAccessKey", cfg.S3.SecretAccessKey, "secret"},
		{"SessionToken", cfg.S3.SessionToken, "FwoGZXIvYXdzEXAMPLE"},
		{"DownloadURL", cfg.S3.DownloadURL, "https://d123.cloudfront.net"},
		{"Prefix", cfg.S3.Prefix, "prod/"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("S3.%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
	if !cfg.S3.UsePathStyle {
		t.Errorf("expected S3.UsePathStyle=true")
	}
}

// TestS3LegacyBucketURLNotRead locks in the clean break from issue #42:
// the deprecated `s3.bucketURL` viper key must not populate the renamed
// `DownloadURL` field. If a future change reintroduces a fallback alias,
// this test will fail and force an explicit decision.
func TestS3LegacyBucketURLNotRead(t *testing.T) {
	const yaml = `
s3:
  bucketURL: https://legacy.example.com
`
	vp := viper.New()
	vp.SetConfigType("yaml")
	if err := vp.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("read config: %v", err)
	}
	cfg := New()
	cfg.ConfigureWithViper(vp)
	if cfg.S3.DownloadURL != "" {
		t.Errorf("legacy s3.bucketURL must not populate DownloadURL; got %q", cfg.S3.DownloadURL)
	}
}

func TestS3CredentialsFromEnv(t *testing.T) {
	const yaml = `
s3:
  accessKeyID: from-yaml-id
  secretAccessKey: from-yaml-secret
  sessionToken: from-yaml-token
`
	tests := []struct {
		name      string
		envs      map[string]string
		wantID    string
		wantSec   string
		wantToken string
	}{
		{
			name:      "no env vars — YAML wins",
			envs:      nil,
			wantID:    "from-yaml-id",
			wantSec:   "from-yaml-secret",
			wantToken: "from-yaml-token",
		},
		{
			name: "TS_ prefixed env overrides YAML",
			envs: map[string]string{
				"TS_S3_ACCESS_KEY_ID":     "ts-id",
				"TS_S3_SECRET_ACCESS_KEY": "ts-secret",
				"TS_S3_SESSION_TOKEN":     "ts-token",
			},
			wantID:    "ts-id",
			wantSec:   "ts-secret",
			wantToken: "ts-token",
		},
		{
			name: "AWS standard env overrides YAML",
			envs: map[string]string{
				"AWS_ACCESS_KEY_ID":     "aws-id",
				"AWS_SECRET_ACCESS_KEY": "aws-secret",
				"AWS_SESSION_TOKEN":     "aws-token",
			},
			wantID:    "aws-id",
			wantSec:   "aws-secret",
			wantToken: "aws-token",
		},
		{
			name: "TS_ prefixed env wins over AWS standard env",
			envs: map[string]string{
				"TS_S3_ACCESS_KEY_ID":     "ts-id",
				"TS_S3_SECRET_ACCESS_KEY": "ts-secret",
				"TS_S3_SESSION_TOKEN":     "ts-token",
				"AWS_ACCESS_KEY_ID":       "aws-id",
				"AWS_SECRET_ACCESS_KEY":   "aws-secret",
				"AWS_SESSION_TOKEN":       "aws-token",
			},
			wantID:    "ts-id",
			wantSec:   "ts-secret",
			wantToken: "ts-token",
		},
	}
	credEnvs := []string{
		"TS_S3_ACCESS_KEY_ID",
		"TS_S3_SECRET_ACCESS_KEY",
		"TS_S3_SESSION_TOKEN",
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清空所有相关 env，避免宿主机/CI runner 上的真实凭据污染断言
			for _, k := range credEnvs {
				t.Setenv(k, "")
			}
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			vp := viper.New()
			vp.SetConfigType("yaml")
			if err := vp.ReadConfig(strings.NewReader(yaml)); err != nil {
				t.Fatalf("read config: %v", err)
			}
			cfg := New()
			cfg.ConfigureWithViper(vp)
			if cfg.S3.AccessKeyID != tt.wantID {
				t.Errorf("S3.AccessKeyID = %q, want %q", cfg.S3.AccessKeyID, tt.wantID)
			}
			if cfg.S3.SecretAccessKey != tt.wantSec {
				t.Errorf("S3.SecretAccessKey = %q, want %q", cfg.S3.SecretAccessKey, tt.wantSec)
			}
			if cfg.S3.SessionToken != tt.wantToken {
				t.Errorf("S3.SessionToken = %q, want %q", cfg.S3.SessionToken, tt.wantToken)
			}
		})
	}
}
