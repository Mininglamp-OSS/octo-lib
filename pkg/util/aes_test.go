package util

import (
	"bytes"
	"testing"
)

func TestPKCS7UnPadding(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   []byte{},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid padding 1",
			input:   []byte{0x01},
			want:    []byte{},
			wantErr: false,
		},
		{
			name:    "valid padding 4",
			input:   []byte{'h', 'e', 'l', 'l', 'o', 0x03, 0x03, 0x03},
			want:    []byte{'h', 'e', 'l', 'l', 'o'},
			wantErr: false,
		},
		{
			name:    "unpadding larger than length",
			input:   []byte{0x05},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "zero padding value",
			input:   []byte{'a', 'b', 0x00},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "inconsistent padding bytes",
			input:   []byte{'h', 'e', 'l', 'l', 'o', 0x02, 0x03, 0x03},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "full block padding",
			input:   []byte{0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10},
			want:    []byte{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PKCS7UnPadding(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PKCS7UnPadding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("PKCS7UnPadding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPKCS5UnPadding(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   []byte{},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid padding",
			input:   []byte{'t', 'e', 's', 't', 0x04, 0x04, 0x04, 0x04},
			want:    []byte{'t', 'e', 's', 't'},
			wantErr: false,
		},
		{
			name:    "unpadding larger than length",
			input:   []byte{0x10},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "zero padding value",
			input:   []byte{'x', 0x00},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "inconsistent padding bytes",
			input:   []byte{'a', 'b', 0x01, 0x02},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PKCS5UnPadding(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PKCS5UnPadding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("PKCS5UnPadding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAesEncryptDecryptRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	plaintext := []byte("Hello, World!")

	encrypted, err := AesEncryptPkcs7(plaintext, key, iv)
	if err != nil {
		t.Fatalf("AesEncryptPkcs7() error = %v", err)
	}

	decrypted, err := AesDecryptPkcs7(encrypted, key, iv)
	if err != nil {
		t.Fatalf("AesDecryptPkcs7() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Roundtrip failed: got %v, want %v", decrypted, plaintext)
	}
}
