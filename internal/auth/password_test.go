package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}
	if err := VerifyPassword("correct horse battery staple", hash); err != nil {
		t.Errorf("valid password should verify: %v", err)
	}
	if err := VerifyPassword("wrong password here!!", hash); err == nil {
		t.Errorf("wrong password should not verify")
	}
}

func TestHashesAreSalted(t *testing.T) {
	h1, _ := HashPassword("same-password-1234")
	h2, _ := HashPassword("same-password-1234")
	if h1 == h2 {
		t.Errorf("expected distinct salts to produce distinct hashes")
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "notahash", "$argon2id$bad"} {
		if err := VerifyPassword("x", bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestCheckPassword(t *testing.T) {
	if ps := CheckPassword("short"); ps.Acceptable {
		t.Errorf("short password should be rejected")
	}
	if ps := CheckPassword("password123"); ps.Acceptable {
		t.Errorf("common password should be rejected")
	}
	if ps := CheckPassword("A-long-enough-passphrase-9"); !ps.Acceptable {
		t.Errorf("strong passphrase should be accepted")
	}
}
