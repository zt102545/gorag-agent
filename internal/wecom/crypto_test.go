package wecom

import "testing"

func TestSecretCodec_RoundTrip(t *testing.T) {
	codec := newSecretCodec("test-master-key")
	encrypted, err := codec.encrypt("sensitive-value")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "sensitive-value" {
		t.Fatal("value was not encrypted")
	}
	plain, err := codec.decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "sensitive-value" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestSecretCodec_RejectsWrongKey(t *testing.T) {
	encrypted, err := newSecretCodec("key-one").encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newSecretCodec("key-two").decrypt(encrypted); err == nil {
		t.Fatal("decrypt succeeded with wrong key")
	}
}
