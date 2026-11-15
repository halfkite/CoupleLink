package api

import "testing"

func TestGeneratedAccessKeyMatchesTheAcceptedPattern(t *testing.T) {
	for index := 0; index < 200; index++ {
		key, err := newAccessKey()
		if err != nil {
			t.Fatal(err)
		}
		if !accessKeyPattern.Match(key) {
			t.Fatalf("generated key %q is rejected by the login pattern", key)
		}
		if len(key) != 23 {
			t.Fatalf("unexpected key length %d for %q", len(key), key)
		}
	}
}

func TestAccessKeyPatternRejectsEverythingElse(t *testing.T) {
	for _, value := range []string{
		"", "key", "keyabcdefgh", "KEYabcdefghijklmnopqrst", "keyabcdefghijklmno1",
		"skeyabcdefghijklmnopqrst", "keyabcdefghijklmnopqrst-",
	} {
		if accessKeyPattern.Match(value) {
			t.Fatalf("unexpected match for %q", value)
		}
	}
}

func TestAccessKeyPatternAcceptsTheDocumentedShape(t *testing.T) {
	if !accessKeyPattern.Match("keyabcdefghijklmnopqrst") {
		t.Fatal("the documented key shape must be accepted")
	}
}