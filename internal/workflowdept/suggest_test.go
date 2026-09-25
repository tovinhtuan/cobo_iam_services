package workflowdept

import "testing"

func TestSuggestionDoesNotAutoWriteSemantic(t *testing.T) {
	if got := ClassifySuggestion("Phòng Pháp chế", "Pháp chế & Tuân thủ"); got != SuggestSemantic && got != SuggestPrefix {
		// "& Tuân thủ" is extra text, not a single prefix drop.
		if got == SuggestExact {
			t.Fatal("semantic name must not be exact")
		}
	}
	if ClassifySuggestion("Phòng Pháp chế", "Pháp chế & Tuân thủ") == SuggestExact {
		t.Fatal("exact")
	}
	if ClassifySuggestion("Phòng Pháp chế", "Pháp chế") != SuggestPrefix {
		t.Fatalf("prefix got %s", ClassifySuggestion("Phòng Pháp chế", "Pháp chế"))
	}
	if ClassifySuggestion("Phòng Pháp chế", "phòng   pháp chế") != SuggestExact {
		t.Fatal("whitespace/case should be exact")
	}
	t.Setenv(EnvBackfillWriteEnabled, "false")
	if AllowAutoWrite(true, SuggestExact, 1) {
		t.Fatal("flag off must not write")
	}
	t.Setenv(EnvBackfillWriteEnabled, "true")
	if AllowAutoWrite(true, SuggestExact, 2) {
		t.Fatal("two candidates must not write")
	}
	if AllowAutoWrite(true, SuggestSemantic, 1) {
		t.Fatal("semantic must not write")
	}
	if !AllowAutoWrite(true, SuggestExact, 1) {
		t.Fatal("single exact with flag should be allowed")
	}
}
