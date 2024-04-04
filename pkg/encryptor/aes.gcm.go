package encryptor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"

	"github.com/forest33/tapir/business/entity"
)

type aesGCM struct {
	key []byte
	gcm cipher.AEAD
}

func NewAESGCM(key string) entity.Encryptor {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		panic(err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}

	return &aesGCM{
		key: []byte(key),
		gcm: gcm,
	}
}

func (e *aesGCM) Decrypt(in any) ([]byte, error) {
	if in == nil {
		return nil, errors.New("nil input")
	}
	data, ok := in.([]byte)
	if !ok {
		return nil, errors.New("invalid input type")
	}

	nonceSize := e.gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func (e *aesGCM) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, entity.ErrEmptyMessagePayload
	}

	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := e.gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

func (e *aesGCM) GetLength(plainLength int) int {
	return plainLength + e.gcm.NonceSize() + e.gcm.Overhead()
}

func (e *aesGCM) SetKey(key []byte) {
	e.key = key
}

func (e *aesGCM) GetKey() string {
	return string(e.key)
}
