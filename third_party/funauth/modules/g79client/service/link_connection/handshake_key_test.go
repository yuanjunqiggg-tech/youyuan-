package link_connection

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

func TestHandshakeR2PreservesBinaryZeroBytesAfterRSA(t *testing.T) {
	priv, e := clientPrivateKey()
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"last-zero", "two-last-zero", "leading-zero", "all-zero", "legacy-padding", "ordinary"} {
		t.Run(name, func(t *testing.T) {
			key := bytes.Repeat([]byte{0x32}, 16)
			switch name {
			case "last-zero":
				key[15] = 0
			case "two-last-zero":
				key[14] = 0
				key[15] = 0
			case "leading-zero":
				key[0] = 0
			case "all-zero":
				clear(key)
			}
			payload := append([]byte{}, key...)
			if name == "legacy-padding" {
				payload = append(payload, make([]byte, 16)...)
			}
			encrypted, e := rsa.EncryptPKCS1v15(rand.Reader, &priv.PublicKey, payload)
			if e != nil {
				t.Fatal(e)
			}
			plain, e := rsaDecrypt(encrypted)
			if e != nil {
				t.Fatal(e)
			}
			got, e := linkCipherR2(plain)
			if e != nil || !bytes.Equal(got, key) {
				t.Fatal("fixed-width key changed", e)
			}
		})
	}
}
func TestHandshakeR2RejectsTruncatedAndNonZeroSuffix(t *testing.T) {
	for _, payload := range [][]byte{nil, make([]byte, 15), append(make([]byte, 16), 1)} {
		if _, e := linkCipherR2(payload); e == nil {
			t.Fatal("malformed key accepted")
		}
	}
	if _, e := rsaDecrypt(make([]byte, 256)); e == nil {
		t.Fatal("invalid RSA ciphertext accepted")
	}
}
