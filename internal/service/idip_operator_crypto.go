package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"starcrystal/server/internal/config"
)

const idipPasswordEncPrefix = "v1:"

var (
	ErrIdipCipherKeyMissing = errors.New("idip operator cipher key not configured")
	ErrIdipPasswordDecrypt  = errors.New("idip operator password decrypt failed")
)

// ResolveIdipOperatorCipherKey from env IDIP_OPERATOR_CIPHER_KEY or config idip.operatorCipherKey (base64, 32 bytes).
func ResolveIdipOperatorCipherKey(cfg config.IdipConfig) ([]byte, error) {
	if v := strings.TrimSpace(os.Getenv("IDIP_OPERATOR_CIPHER_KEY")); v != "" {
		return decodeCipherKey(v)
	}
	if k := strings.TrimSpace(cfg.OperatorCipherKey); k != "" {
		return decodeCipherKey(k)
	}
	return nil, ErrIdipCipherKeyMissing
}

// DecodeCipherKeyForTest exposes decodeCipherKey for api tests.
func DecodeCipherKeyForTest(s string) ([]byte, error) {
	return decodeCipherKey(s)
}

func decodeCipherKey(s string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("cipher key base64: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("cipher key must be 32 bytes, got %d", len(raw))
	}
	return raw, nil
}

// EncryptIdipPassword AES-256-GCM; returns "v1:" + base64(nonce|ciphertext).
func EncryptIdipPassword(plain string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("cipher key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plain), nil)
	out := append(nonce, ct...)
	return idipPasswordEncPrefix + base64.StdEncoding.EncodeToString(out), nil
}

func DecryptIdipPassword(enc string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("cipher key must be 32 bytes")
	}
	enc = strings.TrimSpace(enc)
	if !strings.HasPrefix(enc, idipPasswordEncPrefix) {
		return "", ErrIdipPasswordDecrypt
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(enc, idipPasswordEncPrefix))
	if err != nil {
		return "", ErrIdipPasswordDecrypt
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", ErrIdipPasswordDecrypt
	}
	plain, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", ErrIdipPasswordDecrypt
	}
	return string(plain), nil
}

// VerifyIdipOperator checks username/password against config operators (encrypted or legacy plaintext).
func VerifyIdipOperator(cfg config.IdipConfig, username, password string) bool {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return false
	}
	key, keyErr := ResolveIdipOperatorCipherKey(cfg)
	for _, op := range cfg.Operators {
		if !strings.EqualFold(strings.TrimSpace(op.Username), username) {
			continue
		}
		if enc := strings.TrimSpace(op.PasswordEnc); enc != "" {
			if keyErr != nil {
				return false
			}
			plain, err := DecryptIdipPassword(enc, key)
			if err != nil {
				return false
			}
			return subtle.ConstantTimeCompare([]byte(plain), []byte(password)) == 1
		}
		if plain := strings.TrimSpace(op.Password); plain != "" {
			return subtle.ConstantTimeCompare([]byte(plain), []byte(password)) == 1
		}
		return false
	}
	return false
}
