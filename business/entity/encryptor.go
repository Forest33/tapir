package entity

const (
	EncryptionNone      EncryptorMethod = "none"
	EncryptionAES256ECB EncryptorMethod = "aes-256-ecb"
	EncryptionAES256GCM EncryptorMethod = "aes-256-gcm"
)

type EncryptorMethod string

func (m EncryptorMethod) String() string {
	return string(m)
}

func (m EncryptorMethod) KeySize() int {
	switch m {
	case EncryptionNone:
		return 0
	case EncryptionAES256ECB, EncryptionAES256GCM:
		return 32
	}
	panic("unknown encryption method")
}

type Decoder interface {
	Decrypt(data any) ([]byte, error)
}

type Encoder interface {
	Encrypt(data []byte) ([]byte, error)
	GetLength(int) int
}

type Encryptor interface {
	Decoder
	Encoder
	SetKey([]byte)
	GetKey() string
}
