package encryptor

import (
	"errors"

	"github.com/forest33/tapir/business/entity"
)

type emptyEncryptor struct {
	key []byte
}

func NewEmpty(key string) entity.Encryptor {
	return &emptyEncryptor{
		key: []byte(key),
	}
}

func (_ *emptyEncryptor) Decrypt(in any) ([]byte, error) {
	data, ok := in.([]byte)
	if !ok {
		return nil, errors.New("invalid input type")
	}
	return data, nil
}

func (e *emptyEncryptor) Encrypt(data []byte) ([]byte, error) {
	return data, nil
}

func (_ *emptyEncryptor) GetLength(plainLength int) int {
	return plainLength
}

func (e *emptyEncryptor) SetKey(key []byte) {
	e.key = key
}

func (e *emptyEncryptor) GetKey() string {
	return string(e.key)
}
