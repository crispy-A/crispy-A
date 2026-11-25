package auth

import "testing"

func TestHashPasswordVerifies(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !verifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if verifyPassword(hash, "wrong password") {
		t.Fatal("wrong password must not verify")
	}
}
