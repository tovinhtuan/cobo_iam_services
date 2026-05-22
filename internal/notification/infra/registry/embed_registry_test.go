package registry_test

import (
	"context"
	"testing"
	"testing/fstest"

	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

func TestEmbedRegistry_ResolveFallsBackToDefaultLocale(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()

	resolved, err := registry.Resolve(context.Background(), "auth.password_reset.user", "en")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Locale != "vi" {
		t.Fatalf("expected locale vi, got %q", resolved.Locale)
	}
}

func TestEmbedRegistry_ResolveInvalidMetaFails(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistryFS(fstest.MapFS{
		"bad/meta.yaml":      &fstest.MapFile{Data: []byte(":\n")},
		"bad/vi/subject.txt": &fstest.MapFile{Data: []byte("subject")},
		"bad/vi/body.txt":    &fstest.MapFile{Data: []byte("body")},
	})

	if _, err := registry.Resolve(context.Background(), "bad", "vi"); err == nil {
		t.Fatal("expected resolve error")
	}
}
