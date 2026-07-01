// 独立 AES-256-GCM 加密 CLI，供 idip-webclient/scripts_encrypt/encrypt-idip-operator.sh 调用。
// 构建：scripts_encrypt/build-encrypt-tool.sh → scripts_encrypt/bin/encrypt-idip-operator
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
)

const idipPasswordEncPrefix = "v1:"

func encryptIdipPassword(plain string, key []byte) (string, error) {
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

func main() {
	username := flag.String("username", "", "operator username")
	password := flag.String("password", "", "operator password")
	cipherKeyB64 := flag.String("cipher-key-base64", "", "32-byte AES key, base64")
	flag.Parse()
	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "username and password required")
		os.Exit(2)
	}
	keyB64 := *cipherKeyB64
	if keyB64 == "" {
		keyB64 = os.Getenv("IDIP_OPERATOR_CIPHER_KEY")
	}
	if keyB64 == "" {
		fmt.Fprintln(os.Stderr, "cipher key required: -cipher-key-base64 or IDIP_OPERATOR_CIPHER_KEY")
		os.Exit(2)
	}
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(key) != 32 {
		fmt.Fprintln(os.Stderr, "cipher key must be base64-encoded 32 bytes")
		os.Exit(2)
	}
	enc, err := encryptIdipPassword(*password, key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("{\n  \"username\": %q,\n  \"passwordEnc\": %q\n}\n", *username, enc)
}
