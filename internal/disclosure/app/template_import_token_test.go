package app_test

import (
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestTemplateImportToken_DeterministicPayloadHash(t *testing.T) {
	tpl1 := disclosureapp.TemplateImportDefinitionV1{
		Name:             "BCTC Quý 1",
		TemplateCategory: "periodic",
		DeadlineRule:     "T+20",
		GroupID:          "group-001",
		DisplayGroupCodes: []string{"display_groups_003"},
	}

	tpl2 := disclosureapp.TemplateImportDefinitionV1{
		Name:             "BCTC Quý 1",
		TemplateCategory: "periodic",
		DeadlineRule:     "T+20",
		GroupID:          "group-001",
		DisplayGroupCodes: []string{"display_groups_003"},
	}

	hash1, err1 := disclosureapp.ComputeCanonicalTemplatePayloadHash(&tpl1)
	if err1 != nil {
		t.Fatalf("ComputeCanonicalTemplatePayloadHash 1 failed: %v", err1)
	}

	hash2, err2 := disclosureapp.ComputeCanonicalTemplatePayloadHash(&tpl2)
	if err2 != nil {
		t.Fatalf("ComputeCanonicalTemplatePayloadHash 2 failed: %v", err2)
	}

	if hash1 != hash2 {
		t.Errorf("expected deterministic hash, got %q != %q", hash1, hash2)
	}

	// Changing any field changes the hash
	tpl3 := tpl1
	tpl3.Name = "BCTC Quý 2"
	hash3, _ := disclosureapp.ComputeCanonicalTemplatePayloadHash(&tpl3)
	if hash3 == hash1 {
		t.Error("expected different hash when template name changes")
	}
}

func TestTemplateImportToken_SigningAndVerificationLifecycle(t *testing.T) {
	signer := disclosureapp.NewTemplateImportSigner("test-signing-secret-12345678", 15*time.Minute)
	now := time.Now()
	claims := disclosureapp.TemplateImportTokenClaims{
		SchemaVersion: disclosureapp.TemplateImportSchemaVersion,
		PayloadHash:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		ActorID:       "user-admin-001",
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(15 * time.Minute).Unix(),
	}

	tok, err := signer.IssueToken(claims)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	// 1. Positive: valid token and matching actor
	verified, err := signer.VerifyToken(tok, "user-admin-001", now)
	if err != nil {
		t.Fatalf("VerifyToken failed on valid token: %v", err)
	}
	if verified.PayloadHash != claims.PayloadHash {
		t.Errorf("verified PayloadHash = %q, want %q", verified.PayloadHash, claims.PayloadHash)
	}
	if verified.ActorID != claims.ActorID {
		t.Errorf("verified ActorID = %q, want %q", verified.ActorID, claims.ActorID)
	}

	// 2. Negative: wrong actor
	_, err = signer.VerifyToken(tok, "different-user-999", now)
	if err == nil {
		t.Error("expected error for actor mismatch, got nil")
	}

	// 3. Negative: expired token
	futureTime := now.Add(16 * time.Minute)
	_, err = signer.VerifyToken(tok, "user-admin-001", futureTime)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}

	// 4. Negative: tampered signature
	tamperedTok := tok[:len(tok)-2] + "XX"
	_, err = signer.VerifyToken(tamperedTok, "user-admin-001", now)
	if err == nil {
		t.Error("expected error for tampered signature, got nil")
	}

	// 5. Negative: wrong signer secret
	otherSigner := disclosureapp.NewTemplateImportSigner("different-secret-99999", 15*time.Minute)
	_, err = otherSigner.VerifyToken(tok, "user-admin-001", now)
	if err == nil {
		t.Error("expected error for signature from different secret, got nil")
	}

	// 6. Negative: malformed format (no dot or extra dots)
	_, err = signer.VerifyToken("not-a-token", "user-admin-001", now)
	if err == nil || !strings.Contains(err.Error(), "invalid token format") {
		t.Errorf("expected invalid format error, got %v", err)
	}
}
