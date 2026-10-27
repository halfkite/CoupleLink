package api

import (
	"strings"
	"testing"
)

func TestPasswordHashAndMalformedParameters(t *testing.T) {
	password := "a long private 密码 123"
	encoded, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, password) || !verifyPassword(encoded, password) || verifyPassword(encoded, "incorrect password") {
		t.Fatal("password verification failed")
	}
	for _, bad := range []string{"", encoded + "$extra", strings.Replace(encoded, "m=65536", "m=4294967295", 1), strings.Replace(encoded, "p=2", "p=0", 1)} {
		if verifyPassword(bad, password) {
			t.Fatal("malformed hash accepted")
		}
	}
}
func TestTokensAndIdentifiers(t *testing.T) {
	a, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) != 43 || len(tokenHash(a)) != 64 || secretEqual(a, b) || !secretEqual(a, a) {
		t.Fatal("token generation or comparison failed")
	}
	id, err := identifier()
	if err != nil || !uuidPattern.MatchString(id) {
		t.Fatal("invalid identifier")
	}
}
