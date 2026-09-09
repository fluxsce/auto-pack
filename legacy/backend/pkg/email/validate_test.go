package email

import "testing"

func TestValidateAddress(t *testing.T) {
	if err := ValidateAddress("user@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAddress("Bad"); err == nil {
		t.Fatal("expected error for invalid address")
	}
}
