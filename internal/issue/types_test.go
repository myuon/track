package issue

import "testing"

func TestValidateStatusAndDue(t *testing.T) {
	if err := ValidateStatus("todo"); err != nil {
		t.Fatalf("expected valid status: %v", err)
	}
	if err := ValidateStatus("bad"); err == nil {
		t.Fatalf("expected invalid status error")
	}

	if err := ValidateDue("2026-02-10"); err != nil {
		t.Fatalf("expected valid due: %v", err)
	}
	if err := ValidateDue("2026-13-10"); err == nil {
		t.Fatalf("expected invalid due error")
	}
}
