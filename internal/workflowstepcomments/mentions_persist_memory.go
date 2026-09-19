package workflowstepcomments

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// MemoryMentionsRepository is an in-memory MentionsRepository for unit tests.
// Tenant key is always company_id + comment_id.
type MemoryMentionsRepository struct {
	mu sync.Mutex
	// key = companyID + "\x00" + commentID
	byComment map[string][]Mention

	// FailAfterDelete, when true, causes ReplaceForComment to error after delete
	// and restore the prior snapshot (simulates transaction rollback / atomic replace).
	FailAfterDelete bool
	// FailOnInsertID, when set, errors when inserting that mention id (partial insert → rollback).
	FailOnInsertID string
}

func NewMemoryMentionsRepository() *MemoryMentionsRepository {
	return &MemoryMentionsRepository{byComment: map[string][]Mention{}}
}

func mentionKey(companyID, commentID string) string {
	return strings.TrimSpace(companyID) + "\x00" + strings.TrimSpace(commentID)
}

func (r *MemoryMentionsRepository) ListByComment(_ context.Context, companyID, commentID string) ([]Mention, error) {
	companyID = strings.TrimSpace(companyID)
	commentID = strings.TrimSpace(commentID)
	if companyID == "" || commentID == "" {
		return nil, ErrInvalidMentions{Reason: "company_id and comment_id required"}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	src := r.byComment[mentionKey(companyID, commentID)]
	out := make([]Mention, len(src))
	copy(out, src)
	sortMentions(out)
	return out, nil
}

func (r *MemoryMentionsRepository) ReplaceForComment(_ context.Context, _ DBTX, companyID, commentID string, mentions []Mention) error {
	prepared, err := PrepareMentionsForPersist(companyID, commentID, mentions)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := mentionKey(companyID, commentID)
	prev := append([]Mention(nil), r.byComment[key]...)
	delete(r.byComment, key)
	if r.FailAfterDelete {
		r.byComment[key] = prev
		return fmt.Errorf("simulated replace failure after delete")
	}
	next := make([]Mention, 0, len(prepared))
	for _, m := range prepared {
		if r.FailOnInsertID != "" && m.ID == r.FailOnInsertID {
			r.byComment[key] = prev
			return fmt.Errorf("simulated insert failure for id %s", m.ID)
		}
		next = append(next, m)
	}
	sortMentions(next)
	r.byComment[key] = next
	return nil
}

func (r *MemoryMentionsRepository) DeleteByComment(_ context.Context, _ DBTX, companyID, commentID string) error {
	companyID = strings.TrimSpace(companyID)
	commentID = strings.TrimSpace(commentID)
	if companyID == "" || commentID == "" {
		return ErrInvalidMentions{Reason: "company_id and comment_id required"}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byComment, mentionKey(companyID, commentID))
	return nil
}

// SnapshotMentions copies mention store for memory unit-of-work rollback.
func (r *MemoryMentionsRepository) SnapshotMentions() map[string][]Mention {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string][]Mention, len(r.byComment))
	for k, v := range r.byComment {
		cp := make([]Mention, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// RestoreMentions replaces the mention store (memory rollback).
func (r *MemoryMentionsRepository) RestoreMentions(snap map[string][]Mention) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byComment = snap
}

func sortMentions(items []Mention) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].StartOffset != items[j].StartOffset {
			return items[i].StartOffset < items[j].StartOffset
		}
		if items[i].EndOffset != items[j].EndOffset {
			return items[i].EndOffset < items[j].EndOffset
		}
		return items[i].MentionedMembershipID < items[j].MentionedMembershipID
	})
}
