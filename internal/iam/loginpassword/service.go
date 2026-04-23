package loginpassword

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

// AlgRSAOAEP256 is the only supported envelope for browser Web Crypto RSA-OAEP + SHA-256.
const AlgRSAOAEP256 = "RSA-OAEP-256"

// Service holds an RSA private key used to decrypt login passwords encrypted by the web client.
type Service struct {
	priv *rsa.PrivateKey
	kid  string
}

// NewFromPEM parses a PEM-encoded PKCS#1 or PKCS#8 RSA private key.
func NewFromPEM(pemStr string, kid string) (*Service, error) {
	if pemStr == "" {
		return nil, fmt.Errorf("empty PEM")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("parse RSA private key: %w", err)
		}
		var ok bool
		priv, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}
	if priv.N.BitLen() < 2048 {
		return nil, fmt.Errorf("RSA key must be at least 2048 bits (got %d)", priv.N.BitLen())
	}
	if kid == "" {
		kid = "default"
	}
	return &Service{priv: priv, kid: kid}, nil
}

// KeyID returns the configured key identifier (for client kid field).
func (s *Service) KeyID() string { return s.kid }

// PublicKeySPKIB64 returns PKIX DER of the public key, standard base64 (for Web Crypto spki import).
func (s *Service) PublicKeySPKIB64() (string, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(&s.priv.PublicKey)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pubDER), nil
}

// DecryptOAEP256 decrypts ciphertext produced with RSA-OAEP and SHA-256 (Web Crypto compatible).
func (s *Service) DecryptOAEP256(ciphertextB64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("ciphertext base64: %w", err)
	}
	pt, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, s.priv, raw, nil)
	if err != nil {
		return "", fmt.Errorf("rsa decrypt: %w", err)
	}
	return string(pt), nil
}
