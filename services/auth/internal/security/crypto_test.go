package security

import "testing"

func TestEncryptorEncryptDecrypt(t *testing.T) {
	encryptor, err := NewEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewEncryptor() error = %v", err)
	}

	cipherText, err := encryptor.Encrypt("user@example.com")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if cipherText == "user@example.com" {
		t.Fatal("ciphertext should not equal plaintext")
	}

	plainText, err := encryptor.Decrypt(cipherText)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plainText != "user@example.com" {
		t.Fatalf("Decrypt() = %q, want %q", plainText, "user@example.com")
	}
}
