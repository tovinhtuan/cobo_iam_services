package templatebuilderoauth

import (
	"context"
	"sync"
	"time"
)

// MemoryStore exists for unit tests only. The API server refuses to enable this
// OAuth feature unless it has a durable MySQL grant store.
type MemoryStore struct {
	mu    sync.Mutex
	items map[string]AuthorizationCode
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{items: map[string]AuthorizationCode{}} }

func (s *MemoryStore) CreateAuthorizationCode(_ context.Context, code AuthorizationCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[code.CodeHash] = code
	return nil
}

func (s *MemoryStore) ConsumeAuthorizationCode(_ context.Context, hash string, now time.Time) (*AuthorizationCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[hash]
	if !ok || !item.ExpiresAt.After(now) {
		return nil, nil
	}
	delete(s.items, hash)
	return &item, nil
}
