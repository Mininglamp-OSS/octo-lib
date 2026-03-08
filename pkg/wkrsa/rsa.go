package wkrsa

import (
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
)

// SignWithMD5 rsa签名 (使用MD5哈希)
// pemPrivKey 私钥key 类似 -----BEGIN RSA PRIVATE KEY-----
// xxxx
// -----END RSA PRIVATE KEY-----
//
// Deprecated: MD5 is cryptographically broken and should not be used for secure signatures.
// Use SignWithSHA256 instead for better security.
func SignWithMD5(data []byte, pemPrivKey []byte) (string, error) {
	hashMd5 := md5.Sum(data)
	hashed := hashMd5[:]
	block, _ := pem.Decode(pemPrivKey)
	if block == nil {
		return "", errors.New("private key error")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.MD5, hashed)
	return base64.StdEncoding.EncodeToString(signature), err
}

// SignWithSHA256 rsa签名 (使用SHA256哈希)
// pemPrivKey 私钥key 类似 -----BEGIN RSA PRIVATE KEY-----
// xxxx
// -----END RSA PRIVATE KEY-----
func SignWithSHA256(data []byte, pemPrivKey []byte) (string, error) {
	hashSha256 := sha256.Sum256(data)
	hashed := hashSha256[:]
	block, _ := pem.Decode(pemPrivKey)
	if block == nil {
		return "", errors.New("private key error")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed)
	return base64.StdEncoding.EncodeToString(signature), err
}
