package workflowstepcomments_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wsc "github.com/cobo/cobo_iam_services/internal/workflowstepcomments"
	"github.com/google/uuid"
)

func TestValidateAndCanonicalizeMentions_Table(t *testing.T) {
	midA := uuid.NewString()
	midB := uuid.NewString()
	lookup := &wsc.MemoryMembershipActiveLookup{
		Active: map[string]map[string]bool{
			"c1": {midA: true, midB: true, uuid.NewString(): false},
		},
		OtherCompany: map[string]bool{},
	}
	inactiveID := uuid.NewString()
	lookup.Active["c1"][inactiveID] = false
	crossID := uuid.NewString()
	lookup.OtherCompany[crossID] = true

	vnBody := "Vui lòng kiểm tra @Nguyễn Văn A nhé"
	emojiBody := "hello 👋 @user"
	combining := "e\u0301xample @x" // e + combining acute

	tests := []struct {
		name    string
		body    string
		inputs  []wsc.MentionInput
		lookup  wsc.MembershipActiveLookup
		wantErr bool
		code    perr.Code
	}{
		{name: "empty_body_no_mentions", body: "x", inputs: nil, lookup: lookup},
		{name: "missing_mentions", body: "hello", inputs: nil, lookup: lookup},
		{name: "empty_mentions", body: "hello", inputs: []wsc.MentionInput{}, lookup: lookup},
		{
			name: "body_4000_ok",
			body: strings.Repeat("a", 4000),
			inputs: []wsc.MentionInput{{MembershipID: midA, Start: 0, End: 1}},
			lookup: lookup,
		},
		{
			name:    "over_20",
			body:    strings.Repeat("x", 100),
			inputs:  manyMentions(21, midA),
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name: "duplicate",
			body: "abcdef",
			inputs: []wsc.MentionInput{
				{MembershipID: midA, Start: 0, End: 2},
				{MembershipID: midA, Start: 0, End: 2},
			},
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name:    "invalid_uuid",
			body:    "hello",
			inputs:  []wsc.MentionInput{{MembershipID: "not-a-uuid", Start: 0, End: 1}},
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name:    "inactive",
			body:    "hello",
			inputs:  []wsc.MentionInput{{MembershipID: inactiveID, Start: 0, End: 1}},
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name:    "cross_tenant",
			body:    "hello",
			inputs:  []wsc.MentionInput{{MembershipID: crossID, Start: 0, End: 1}},
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name:    "unknown",
			body:    "hello",
			inputs:  []wsc.MentionInput{{MembershipID: uuid.NewString(), Start: 0, End: 1}},
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name:    "negative_offset",
			body:    "hello",
			inputs:  []wsc.MentionInput{{MembershipID: midA, Start: -1, End: 1}},
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name:    "end_le_start",
			body:    "hello",
			inputs:  []wsc.MentionInput{{MembershipID: midA, Start: 2, End: 2}},
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name:    "end_over_body",
			body:    "hi",
			inputs:  []wsc.MentionInput{{MembershipID: midA, Start: 0, End: 5}},
			lookup:  lookup,
			wantErr: true,
			code:    perr.CodeInvalidRequest,
		},
		{
			name: "vietnamese",
			body: vnBody,
			inputs: []wsc.MentionInput{{
				MembershipID: midA,
				Start:        utf8.RuneCountInString("Vui lòng kiểm tra "),
				End:          utf8.RuneCountInString("Vui lòng kiểm tra @Nguyễn Văn A"),
			}},
			lookup: lookup,
		},
		{
			name: "emoji",
			body: emojiBody,
			inputs: []wsc.MentionInput{{
				MembershipID: midA,
				Start:        utf8.RuneCountInString("hello 👋 "),
				End:          utf8.RuneCountInString(emojiBody),
			}},
			lookup: lookup,
		},
		{
			name: "combining",
			body: combining,
			inputs: []wsc.MentionInput{{
				MembershipID: midA,
				Start:        utf8.RuneCountInString("e\u0301xample "),
				End:          utf8.RuneCountInString(combining),
			}},
			lookup: lookup,
		},
		{
			name: "self_mention_ok",
			body: "ping @me",
			inputs: []wsc.MentionInput{{
				MembershipID: midA, Start: 5, End: 8,
			}},
			lookup: lookup,
		},
		{
			name: "sort_canonical",
			body: "abcdefghij",
			inputs: []wsc.MentionInput{
				{MembershipID: midB, Start: 5, End: 7},
				{MembershipID: midA, Start: 1, End: 3},
			},
			lookup: lookup,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			can, js, err := wsc.ValidateAndCanonicalizeMentions(context.Background(), "c1", tc.body, tc.inputs, tc.lookup)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				he, ok := err.(*perr.HTTPError)
				if !ok {
					t.Fatalf("want HTTPError got %T %v", err, err)
				}
				if he.Code != tc.code {
					t.Fatalf("code=%s want %s", he.Code, tc.code)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if js == "" {
				t.Fatal("canonical json required")
			}
			if tc.name == "sort_canonical" {
				if len(can) != 2 || can[0].MembershipID != midA {
					t.Fatalf("expected sorted by start: %+v", can)
				}
			}
			if strings.Contains(js, "display_name") {
				t.Fatal("canonical json must not include display_name")
			}
		})
	}
}

func manyMentions(n int, mid string) []wsc.MentionInput {
	out := make([]wsc.MentionInput, n)
	for i := 0; i < n; i++ {
		out[i] = wsc.MentionInput{MembershipID: mid, Start: i, End: i + 1}
	}
	return out
}

func TestValidate_HTTPStatus(t *testing.T) {
	_, _, err := wsc.ValidateAndCanonicalizeMentions(context.Background(), "c1", "ab", []wsc.MentionInput{
		{MembershipID: "bad", Start: 0, End: 1},
	}, &wsc.MemoryMembershipActiveLookup{})
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400 got %v", err)
	}
}
