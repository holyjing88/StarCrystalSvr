// encrypt-idip-operator 生成 starcrystal.json 中 idip.operators[].passwordEnc。
//
//	go run ./cmd/encrypt-idip-operator -username ops_admin -password 'secret' -cipher-key-base64 '<32-byte-b64>'
package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"starcrystal/server/internal/service"
)

func main() {
	username := flag.String("username", "", "operator username")
	password := flag.String("password", "", "operator password (plaintext, not stored in shell history if passed via file)")
	cipherKeyB64 := flag.String("cipher-key-base64", "", "32-byte AES key, base64 (or set IDIP_OPERATOR_CIPHER_KEY)")
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
	enc, err := service.EncryptIdipPassword(*password, key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf(`{
  "username": %q,
  "passwordEnc": %q
}
`, *username, enc)
}
