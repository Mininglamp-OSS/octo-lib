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
  bucketURL: https://my-bucket.s3.us-west-2.amazonaws.com
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
		{"BucketURL", cfg.S3.BucketURL, "https://my-bucket.s3.us-west-2.amazonaws.com"},
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
