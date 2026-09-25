package workflowdept

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// EncryptRecipientEmails seals addresses with AES-GCM.
// Key material stays outside the database. AAD binds company, occurrence, and resolution id.
func EncryptRecipientEmails(key []byte, keyVersion uint32, companyID, occurrenceID, resolutionID string, plaintext []byte) (ciphertext []byte, err error) {
	if len(key) != 32 {
		return nil, errors.New("recipient key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, aad(companyID, occurrenceID, resolutionID))
	out := make([]byte, 4+len(nonce)+len(sealed))
	binary.BigEndian.PutUint32(out[:4], keyVersion)
	copy(out[4:], nonce)
	copy(out[4+len(nonce):], sealed)
	return out, nil
}

func DecryptRecipientEmails(key []byte, wantVersion uint32, companyID, occurrenceID, resolutionID string, blob []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("recipient key must be 32 bytes")
	}
	if len(blob) < 4 {
		return nil, errors.New("recipient ciphertext missing")
	}
	ver := binary.BigEndian.Uint32(blob[:4])
	if ver != wantVersion {
		return nil, fmt.Errorf("recipient key version %d does not match %d", ver, wantVersion)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(blob) < 4+ns {
		return nil, errors.New("recipient ciphertext missing")
	}
	nonce := blob[4 : 4+ns]
	sealed := blob[4+ns:]
	plain, err := gcm.Open(nil, nonce, sealed, aad(companyID, occurrenceID, resolutionID))
	if err != nil {
		return nil, errors.New("recipient ciphertext rejected")
	}
	return plain, nil
}

func aad(companyID, occurrenceID, resolutionID string) []byte {
	return []byte(companyID + "|" + occurrenceID + "|" + resolutionID)
}
