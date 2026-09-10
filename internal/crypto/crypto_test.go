package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestKeyPairGeneration(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	if kp.PrivateKey == [KeySize]byte{} {
		t.Error("private key is zero")
	}
	if kp.PublicKey == [KeySize]byte{} {
		t.Error("public key is zero")
	}
}

func TestSessionEncryptDecrypt(t *testing.T) {
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()

	// Alice creates session with Bob's public key
	aliceSession, err := NewSession(alice, bob.PublicKey)
	if err != nil {
		t.Fatalf("NewSession(alice): %v", err)
	}

	// Bob creates session with Alice's public key
	bobSession, err := NewSession(bob, alice.PublicKey)
	if err != nil {
		t.Fatalf("NewSession(bob): %v", err)
	}

	// Verify shared keys match
	if aliceSession.SharedKey() != bobSession.SharedKey() {
		t.Fatal("shared keys do not match")
	}

	// Encrypt on Alice's side, decrypt on Bob's side
	plaintext := []byte("hello, this is a secret message for the Vesper framework")
	encrypted, err := aliceSession.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := bobSession.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("round-trip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestMessageEnvelope(t *testing.T) {
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()
	aliceSession, _ := NewSession(alice, bob.PublicKey)
	bobSession, _ := NewSession(bob, alice.PublicKey)

	msg := []byte("framed transport test")
	frame, err := EncodeMessage(aliceSession, msg)
	if err != nil {
		t.Fatalf("EncodeMessage: %v", err)
	}

	decoded, err := DecodeMessage(bobSession, frame)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	if !bytes.Equal(msg, decoded) {
		t.Fatalf("frame round-trip failed: got %q, want %q", decoded, msg)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()
	eve, _ := GenerateKeyPair()

	aliceSession, _ := NewSession(alice, bob.PublicKey)
	// Eve tries to decrypt with her own session
	eveSession, _ := NewSession(eve, bob.PublicKey)

	encrypted, _ := aliceSession.Encrypt([]byte("secret"))

	_, err := eveSession.Decrypt(encrypted)
	if err == nil {
		t.Fatal("decryption should have failed with wrong key")
	}
}

func TestDeriveKey(t *testing.T) {
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()
	aliceSession, _ := NewSession(alice, bob.PublicKey)
	bobSession, _ := NewSession(bob, alice.PublicKey)

	ctx := []byte("vesper-agent-subkey")

	key1, err := DeriveKey(&aliceSession.sharedKey, ctx)
	if err != nil {
		t.Fatalf("DeriveKey(alice): %v", err)
	}

	key2, err := DeriveKey(&bobSession.sharedKey, ctx)
	if err != nil {
		t.Fatalf("DeriveKey(bob): %v", err)
	}

	if key1 != key2 {
		t.Fatal("derived keys do not match")
	}
}

func BenchmarkEncrypt(b *testing.B) {
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()
	session, _ := NewSession(alice, bob.PublicKey)

	plaintext := make([]byte, 4096)
	rand.Read(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.Encrypt(plaintext)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()
	aliceSession, _ := NewSession(alice, bob.PublicKey)
	bobSession, _ := NewSession(bob, alice.PublicKey)

	plaintext := make([]byte, 4096)
	encrypted, _ := aliceSession.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bobSession.Decrypt(encrypted)
	}
}
