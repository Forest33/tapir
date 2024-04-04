package usecase

import (
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/encryptor"
)

var Encryptors = map[entity.EncryptorMethod]func(string) entity.Encryptor{
	entity.EncryptionNone:      encryptor.NewEmpty,
	entity.EncryptionAES256ECB: encryptor.NewAESECB,
	entity.EncryptionAES256GCM: encryptor.NewAESGCM,
}

func GetEncryptor(key string, method entity.EncryptorMethod) entity.Encryptor {
	if enc, ok := Encryptors[method]; ok {
		return enc(key)
	}
	panic(entity.ErrUnknownEncryptionMethod.Error())
}
