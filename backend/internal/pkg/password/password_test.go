package password

import "testing"

func TestToBizErrorReturnsStructuredPasswordPolicyError(t *testing.T) {
	err := ValidatePasswordPolicy("password1!", 10, true, true, true, true)
	bizErr := ToBizError(err)
	if bizErr == nil {
		t.Fatal("expected biz error, got nil")
	}
	if bizErr.Key != "password.needUppercase" {
		t.Fatalf("expected password.needUppercase, got %q", bizErr.Key)
	}

	err = ValidatePasswordPolicy("Pwd1!", 8, true, true, true, true)
	bizErr = ToBizError(err)
	if bizErr == nil {
		t.Fatal("expected biz error for short password, got nil")
	}
	if bizErr.Key != "password.tooShort" {
		t.Fatalf("expected password.tooShort, got %q", bizErr.Key)
	}
	if got := bizErr.Params["n"]; got != 8 {
		t.Fatalf("expected min length param 8, got %#v", got)
	}
}
