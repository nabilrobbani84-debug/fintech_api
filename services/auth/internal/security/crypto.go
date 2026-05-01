package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

type Encryptor struct {
	aead cipher.AEAD
	key  []byte
}

func NewEncryptor(key []byte) (*Encryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Encryptor{aead: aead, key: key}, nil
}

func (e *Encryptor) Encrypt(plainText string) (string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	cipherText := e.aead.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func (e *Encryptor) Decrypt(encoded string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := e.aead.NonceSize()
	if len(cipherText) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce := cipherText[:nonceSize]
	payload := cipherText[nonceSize:]
	plainText, err := e.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}

func (e *Encryptor) LookupHash(value string) string {
	mac := hmac.New(sha256.New, e.key)
	mac.Write([]byte(NormalizeEmail(value)))
	return hex.EncodeToString(mac.Sum(nil))
}

func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
