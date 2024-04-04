package encryptor

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"math"

	"github.com/forest33/tapir/business/entity"
)

type aesECB struct {
	key   []byte
	block cipher.Block
	enc   *ecbEncrypter
	dec   cipher.BlockMode
}

func NewAESECB(key string) entity.Encryptor {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		panic(err)
	}

	return &aesECB{
		key:   []byte(key),
		block: block,
		enc:   (*ecbEncrypter)(newECB(block)),
		dec:   NewECBDecrypter(block),
	}
}

func (e *aesECB) Decrypt(in any) ([]byte, error) {
	if in == nil {
		return nil, errors.New("nil input")
	}
	data, ok := in.([]byte)
	if !ok {
		return nil, errors.New("invalid input type")
	}

	var err error
	origData := make([]byte, len(data))
	e.dec.CryptBlocks(origData, data)
	origData, err = PKCS5UnPadding(origData)

	return origData, err
}

type ecbDecrypter ecb

func NewECBDecrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbDecrypter)(newECB(b))
}

func (x *ecbDecrypter) BlockSize() int { return x.blockSize }

func (x *ecbDecrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("crypto / cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto / cipher: output smaller than input")
	}
	for len(src) > 0 {
		x.b.Decrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}

func (e *aesECB) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, entity.ErrEmptyMessagePayload
	}

	data = PKCS5Padding(data, e.block.BlockSize())
	encrypted := make([]byte, len(data))
	e.enc.CryptBlocks(encrypted, data)

	return encrypted, nil
}

func (_ *aesECB) GetLength(plainLength int) int {
	return int(float64(plainLength) + aes.BlockSize - math.Mod(float64(plainLength), aes.BlockSize))
}

func (e *aesECB) SetKey(key []byte) {
	e.key = key
}

func (e *aesECB) GetKey() string {
	return string(e.key)
}

func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS5UnPadding(in []byte) ([]byte, error) {
	length := len(in)
	unpadding := int(in[length-1])
	if unpadding > length {
		return nil, errors.New("AES unpadding error")
	}
	return in[:(length - unpadding)], nil
}

type ecb struct {
	b         cipher.Block
	blockSize int
}

type ecbEncrypter ecb

func newECB(b cipher.Block) *ecb {
	return &ecb{
		b:         b,
		blockSize: b.BlockSize(),
	}
}

func (x *ecbEncrypter) BlockSize() int {
	return x.blockSize
}

func (x *ecbEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("crypto / cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto / cipher: output smaller than input")
	}
	for len(src) > 0 {
		x.b.Encrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}
