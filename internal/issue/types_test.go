package issue

import "testing"

func TestValidateStatusAndDue(t *testing.T) {
	if err := ValidateTitle("hello"); err != nil {
		t.Fatalf("expected valid title: %v", err)
	}
	if err := ValidateTitle(" \t "); err == nil {
		t.Fatalf("expected invalid empty title error")
	}

	if err := ValidateStatus("todo"); err != nil {
		t.Fatalf("expected valid status: %v", err)
	}
	if err := ValidateStatus("ready"); err != nil {
		t.Fatalf("expected valid status: %v", err)
	}
	if err := ValidateStatus("bad"); err == nil {
		t.Fatalf("expected invalid status error")
	}

	if err := ValidatePriority("none"); err != nil {
		t.Fatalf("expected valid priority none: %v", err)
	}
	if err := ValidatePriority("bad"); err == nil {
		t.Fatalf("expected invalid priority error")
	}

	if err := ValidateDue("2026-02-10"); err != nil {
		t.Fatalf("expected valid due: %v", err)
	}
	if err := ValidateDue("2026-13-10"); err == nil {
		t.Fatalf("expected invalid due error")
	}
}
