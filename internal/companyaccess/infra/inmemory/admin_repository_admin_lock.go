package inmemory

import (
	"context"
	"sync"
)

// LockCompanyAdmins holds a per-company mutex (parity with the MySQL named lock).
func (r *AdminRepository) LockCompanyAdmins(_ context.Context, companyID string) (func(), error) {
	v, _ := r.adminLocks.LoadOrStore(companyID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	var once sync.Once
	return func() { once.Do(mu.Unlock) }, nil
}
