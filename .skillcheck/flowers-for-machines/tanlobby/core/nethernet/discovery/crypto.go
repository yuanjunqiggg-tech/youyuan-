package discovery

import (
	"crypto/aes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/andreburgaud/crypt2go/ecb"
	"github.com/andreburgaud/crypt2go/padding"
)

var key = sha256.Sum256(binary.LittleEndian.AppendUint64(nil, 0xdeadbeef)) // 0xdeadbeef is also referenced as Application ID

func encrypt(src []byte) []byte {
	block, _ := aes.NewCipher(key[:])
	mode := ecb.NewECBEncrypter(block)
	pkcs7 := padding.NewPkcs7Padding(block.BlockSize())
	src, _ = pkcs7.Pad(src)
	dst := make([]byte, len(src))
	mode.CryptBlocks(dst, src)
	return dst
}

func decrypt(src []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("创建数据块失败：%w", err)
	}
	mode := ecb.NewECBDecrypter(block)
	dst := make([]byte, len(src))
	mode.CryptBlocks(dst, src)
	pkcs7 := padding.NewPkcs7Padding(block.BlockSize())
	dst, err = pkcs7.Unpad(dst)
	if err != nil {
		return nil, fmt.Errorf("去填充失败：%w", err)
	}
	return dst, nil
}
