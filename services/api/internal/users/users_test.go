package users

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	email, err := NormalizeEmail("  Ada@Example.COM ")
	if err != nil || email != "ada@example.com" {
		t.Fatalf("got %q, %v", email, err)
	}
	for _, bad := range []string{"", "no-at-sign", "a@b", "a @b.com", "a@b com", "@example.com", strings.Repeat("a", 250) + "@x.com"} {
		if _, err := NormalizeEmail(bad); err == nil {
			t.Fatalf("email %q must fail", bad)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("short password must fail")
	}
	if err := ValidatePassword("correct-horse-12"); err != nil {
		t.Fatalf("valid password: %v", err)
	}
	if err := ValidatePassword(strings.Repeat("a", 73)); err == nil {
		t.Fatal("overlong password must fail (bcrypt truncation)")
	}
}

func TestValidateWorkspaceName(t *testing.T) {
	if _, err := ValidateWorkspaceName("  "); err == nil {
		t.Fatal("blank name must fail")
	}
	if _, err := ValidateWorkspaceName("Acme"); err != nil {
		t.Fatalf("valid name: %v", err)
	}
}

func TestValidateRole(t *testing.T) {
	for _, role := range []string{"owner", "admin", "viewer"} {
		if _, err := ValidateRole(role); err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
	}
	if _, err := ValidateRole("superadmin"); err == nil {
		t.Fatal("unknown role must fail")
	}
}
