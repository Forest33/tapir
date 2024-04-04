package compression

import (
	"bytes"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
	"github.com/rasky/go-lzo"

	"github.com/forest33/tapir/business/entity"
)

type Compressor struct {
	cfg         *Config
	zstdEncoder map[entity.CompressionLevel]*zstd.Encoder
	zstdDecoder *zstd.Decoder
}

type Config struct {
	PayloadSize int
}

func New(cfg *Config) *Compressor {
	zstdEncoder := make(map[entity.CompressionLevel]*zstd.Encoder, 4)
	zstdDecoder, _ := zstd.NewReader(nil)

	for l := zstd.SpeedFastest; l <= zstd.SpeedBestCompression; l++ {
		enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(l))
		zstdEncoder[entity.CompressionLevel(l)] = enc
	}

	return &Compressor{
		cfg:         cfg,
		zstdEncoder: zstdEncoder,
		zstdDecoder: zstdDecoder,
	}
}

func (c *Compressor) CompressLZ4(in []byte) ([]byte, bool) {
	buf := make([]byte, lz4.CompressBlockBound(len(in)))

	n, err := lz4.CompressBlock(in, buf, nil)
	if err != nil {
		return in, false
	}
	if n >= len(in) {
		return in, false
	}

	return buf[:n], true
}

func (c *Compressor) DecompressLZ4(in []byte) ([]byte, error) {
	out := make([]byte, c.cfg.PayloadSize)

	n, err := lz4.UncompressBlock(in, out)
	if err != nil {
		return nil, err
	}

	return out[:n], nil
}

func (c *Compressor) CompressLZO(in []byte) ([]byte, bool) {
	out := lzo.Compress1X(in)
	if len(out) >= len(in) {
		return in, false
	}
	return out, true
}

func (c *Compressor) DecompressLZO(in []byte) ([]byte, error) {
	return lzo.Decompress1X(bytes.NewBuffer(in), len(in), c.cfg.PayloadSize)
}

func (c *Compressor) CompressZSTD(in []byte, level entity.CompressionLevel) ([]byte, bool) {
	if level < 1 || level > 4 {
		level = 2
	}
	out := make([]byte, 0, len(in))
	out = c.zstdEncoder[level].EncodeAll(in, out)
	return out, true
}

func (c *Compressor) DecompressZSTD(in []byte) ([]byte, error) {
	out := make([]byte, 0, c.cfg.PayloadSize)
	out, err := c.zstdDecoder.DecodeAll(in, out)
	if err != nil {
		return in, err
	}
	return out, nil
}
