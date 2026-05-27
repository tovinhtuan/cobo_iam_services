package mediaupload

import (
	"testing"
	"time"
)

func TestSigner_VerifyValidUpload(t *testing.T) {
	s := NewSigner("test-secret", time.Minute)
	exp := time.Now().UTC().Add(5 * time.Minute)
	in := SignInput{
		Purpose:     PurposeUserAvatarUpload,
		OwnerID:     "u_1",
		AssetID:     "asset_1",
		Method:      "PUT",
		Path:        "/api/v1/me/avatar/upload/asset_1",
		ContentType: "image/png",
		SizeBytes:   1024,
		ExpiresAt:   exp,
	}
	sig := s.Sign(in)
	if !s.Verify(sig, in) {
		t.Fatal("expected valid signature")
	}
}

func TestSigner_Expired(t *testing.T) {
	exp := time.Now().UTC().Add(-time.Minute)
	in := SignInput{
		Purpose:     PurposeUserAvatarUpload,
		OwnerID:     "u_1",
		AssetID:     "asset_1",
		Method:      "PUT",
		Path:        "/api/v1/me/avatar/upload/asset_1",
		ContentType: "image/png",
		SizeBytes:   1024,
		ExpiresAt:   exp,
	}
	if VerifyNotExpired(in, time.Now().UTC()) {
		t.Fatal("expected expired")
	}
}

func TestSigner_WrongMethod(t *testing.T) {
	s := NewSigner("test-secret", time.Minute)
	exp := time.Now().UTC().Add(5 * time.Minute)
	in := SignInput{
		Purpose:     PurposeUserAvatarUpload,
		OwnerID:     "u_1",
		AssetID:     "asset_1",
		Method:      "PUT",
		Path:        "/api/v1/me/avatar/upload/asset_1",
		ContentType: "image/png",
		SizeBytes:   1024,
		ExpiresAt:   exp,
	}
	sig := s.Sign(in)
	bad := in
	bad.Method = "GET"
	if s.Verify(sig, bad) {
		t.Fatal("expected method mismatch to fail verify")
	}
}

func TestSigner_WrongPath(t *testing.T) {
	s := NewSigner("test-secret", time.Minute)
	exp := time.Now().UTC().Add(5 * time.Minute)
	in := SignInput{
		Purpose:     PurposeUserAvatarUpload,
		OwnerID:     "u_1",
		AssetID:     "asset_1",
		Method:      "PUT",
		Path:        "/api/v1/me/avatar/upload/asset_1",
		ContentType: "image/png",
		SizeBytes:   1024,
		ExpiresAt:   exp,
	}
	sig := s.Sign(in)
	bad := in
	bad.Path = "/api/v1/me/avatar/upload/other"
	if s.Verify(sig, bad) {
		t.Fatal("expected path mismatch to fail verify")
	}
}

func TestSigner_WrongOwner(t *testing.T) {
	s := NewSigner("test-secret", time.Minute)
	exp := time.Now().UTC().Add(5 * time.Minute)
	in := SignInput{
		Purpose:     PurposeUserAvatarUpload,
		OwnerID:     "u_1",
		AssetID:     "asset_1",
		Method:      "PUT",
		Path:        "/api/v1/me/avatar/upload/asset_1",
		ContentType: "image/png",
		SizeBytes:   1024,
		ExpiresAt:   exp,
	}
	sig := s.Sign(in)
	bad := in
	bad.OwnerID = "u_2"
	if s.Verify(sig, bad) {
		t.Fatal("expected owner mismatch to fail verify")
	}
}

func TestSigner_WrongPurpose(t *testing.T) {
	s := NewSigner("test-secret", time.Minute)
	exp := time.Now().UTC().Add(5 * time.Minute)
	upload := SignInput{
		Purpose:     PurposeUserAvatarUpload,
		OwnerID:     "u_1",
		AssetID:     "asset_1",
		Method:      "PUT",
		Path:        "/api/v1/me/avatar/upload/asset_1",
		ContentType: "image/png",
		SizeBytes:   1024,
		ExpiresAt:   exp,
	}
	sig := s.Sign(upload)
	content := upload
	content.Purpose = PurposeUserAvatarContent
	content.Method = "GET"
	content.Path = "/api/v1/me/avatar/content"
	content.SizeBytes = 0
	content.ContentType = ""
	if s.Verify(sig, content) {
		t.Fatal("expected purpose mismatch to fail verify")
	}
}

func TestSigner_ContentGET(t *testing.T) {
	s := NewSigner("test-secret", time.Minute)
	exp := time.Now().UTC().Add(5 * time.Minute)
	in := SignInput{
		Purpose:   PurposeUserAvatarContent,
		OwnerID:   "u_1",
		AssetID:   "",
		Method:    "GET",
		Path:      "/api/v1/me/avatar/content",
		ExpiresAt: exp,
	}
	sig := s.Sign(in)
	if !s.Verify(sig, in) {
		t.Fatal("expected valid content signature")
	}
}
