package loginpassword

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

func TestRoundTrip_PKCS1(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	blk := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}
	pemStr := string(pem.EncodeToMemory(blk))

	svc, err := NewFromPEM(pemStr, "t1")
	if err != nil {
		t.Fatal(err)
	}
	spki, err := svc.PublicKeySPKIB64()
	if err != nil || spki == "" {
		t.Fatalf("spki: %v", err)
	}

	plain := "my-s3cret-p@ss"
	ct, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &priv.PublicKey, []byte(plain), nil)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(ct)

	got, err := svc.DecryptOAEP256(b64)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("want %q got %q", plain, got)
	}
}

func TestRejectSmallKey(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	blk := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}
	pemStr := string(pem.EncodeToMemory(blk))
	_, err = NewFromPEM(pemStr, "x")
	if err == nil {
		t.Fatal("expected error for 1024-bit key")
	}
}
