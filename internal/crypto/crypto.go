// Package crypto provides shared X25519 + XChaCha20-Poly1305 AEAD encryption
// used by Pulse-C2, Agent, and all Go components of the Vesper Framework.
package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

const (
	KeySize   = 32
	NonceSize = chacha20poly1305.NonceSizeX
	TagSize   = chacha20poly1305.Overhead
)

// KeyPair holds an X25519 keypair for ECDH key exchange.
type KeyPair struct {
	PrivateKey [KeySize]byte
	PublicKey  [KeySize]byte
}

// GenerateKeyPair creates a new X25519 ECDH keypair.
func GenerateKeyPair() (*KeyPair, error) {
	var kp KeyPair

	if _, err := io.ReadFull(rand.Reader, kp.PrivateKey[:]); err != nil {
		return nil, fmt.Errorf("generating private key: %w", err)
	}

	// Clamp the private key per RFC 7748
	kp.PrivateKey[0] &= 248
	kp.PrivateKey[31] &= 127
	kp.PrivateKey[31] |= 64

	curve25519.ScalarBaseMult(&kp.PublicKey, &kp.PrivateKey)

	return &kp, nil
}

// Session represents an encrypted session between two peers.
type Session struct {
	LocalKey  *KeyPair
	RemotePub [KeySize]byte
	sharedKey [KeySize]byte
	aead      cipher.AEAD
}

// NewSession creates an encrypted session given local keypair and remote's public key.
func NewSession(localKey *KeyPair, remotePub [KeySize]byte) (*Session, error) {
	s := &Session{
		LocalKey:  localKey,
		RemotePub: remotePub,
	}

	// X25519 ECDH: derive shared secret
	shared, err := curve25519.X25519(localKey.PrivateKey[:], remotePub[:])
	if err != nil {
		return nil, fmt.Errorf("x25519 exchange: %w", err)
	}
	copy(s.sharedKey[:], shared)

	aead, err := chacha20poly1305.NewX(s.sharedKey[:])
	if err != nil {
		return nil, fmt.Errorf("creating aead: %w", err)
	}
	s.aead = aead

	return s, nil
}

// SharedKey returns the derived X25519 shared secret.
func (s *Session) SharedKey() [KeySize]byte {
	return s.sharedKey
}

// Encrypt encrypts plaintext with XChaCha20-Poly1305 AEAD.
// Returns nonce + ciphertext (including 24-byte nonce prepended).
func (s *Session) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := s.aead.Seal(nil, nonce, plaintext, nil)

	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// Decrypt decrypts a message encrypted with Encrypt (nonce + ciphertext).
func (s *Session) Decrypt(message []byte) ([]byte, error) {
	if len(message) < s.aead.NonceSize()+s.aead.Overhead() {
		return nil, errors.New("message too short")
	}

	nonce := message[:s.aead.NonceSize()]
	ciphertext := message[s.aead.NonceSize():]

	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// MessageEnvelope wraps encrypted data with a length prefix for stream transport.
type MessageEnvelope struct {
	Length uint16
	Data   []byte
}

// EncodeMessage creates a framed message (2-byte length prefix + encrypted data).
func EncodeMessage(session *Session, plaintext []byte) ([]byte, error) {
	encrypted, err := session.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}

	length := uint16(len(encrypted))
	buf := make([]byte, 2+len(encrypted))
	binary.BigEndian.PutUint16(buf, length)
	copy(buf[2:], encrypted)

	return buf, nil
}

// DecodeMessage decodes a framed message.
func DecodeMessage(session *Session, frame []byte) ([]byte, error) {
	if len(frame) < 2 {
		return nil, errors.New("frame too short for length prefix")
	}

	length := binary.BigEndian.Uint16(frame[:2])
	if int(length) != len(frame)-2 {
		return nil, fmt.Errorf("length mismatch: expected %d, got %d", length, len(frame)-2)
	}

	return session.Decrypt(frame[2:])
}

// DeriveKey derives a sub-key from a shared secret using HKDF-like construction.
func DeriveKey(sharedSecret *[KeySize]byte, context []byte) ([KeySize]byte, error) {
	var derived [KeySize]byte

	aead, err := chacha20poly1305.NewX(sharedSecret[:])
	if err != nil {
		return derived, err
	}

	nonce := make([]byte, aead.NonceSize())
	derivedSlice := aead.Seal(nil, nonce, context, nil)

	if len(derivedSlice) < KeySize {
		return derived, errors.New("derived key too short")
	}

	copy(derived[:], derivedSlice[:KeySize])
	return derived, nil
}
