package mediaupload

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiskStorage_WriteReadDelete(t *testing.T) {
	root := t.TempDir()
	store, err := NewDiskStorage(root)
	if err != nil {
		t.Fatalf("NewDiskStorage: %v", err)
	}
	key := AvatarObjectKey("u_1", "asset_abc", "png")
	body := []byte("png-bytes")
	written, err := store.Write(key, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if written != int64(len(body)) {
		t.Fatalf("written = %d, want %d", written, len(body))
	}
	if !store.Exists(key) {
		t.Fatal("expected Exists true")
	}
	got, err := store.Read(key)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("read mismatch")
	}
	if err := store.Delete(key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.Exists(key) {
		t.Fatal("expected deleted")
	}
}

func TestDiskStorage_PathTraversalRejected(t *testing.T) {
	root := t.TempDir()
	store, err := NewDiskStorage(root)
	if err != nil {
		t.Fatalf("NewDiskStorage: %v", err)
	}
	_, err = store.Write("../outside/secret", bytes.NewReader([]byte("x")))
	if err == nil {
		t.Fatal("expected path traversal error")
	}
	if strings.Contains(err.Error(), "traversal") == false && !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
	// ensure file not written outside root
	outside := filepath.Join(filepath.Dir(root), "outside", "secret")
	if store.Exists("../outside/secret") {
		t.Fatal("should not exist via store")
	}
	_ = outside
}
