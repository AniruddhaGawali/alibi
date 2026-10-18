package user

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif" // register decoders for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/webp" // also registers the WebP decoder
)

const (
	avatarSize    = 256     // processed avatars are avatarSize x avatarSize
	webpQuality   = 80      // balance of size and fidelity for a small avatar
	maxSourcePx   = 4096    // longest accepted side, checked on the header before decoding
	maxSourceArea = 8 << 20 // pixels (~8 MP, ~32 MB decoded); guards against decompression bombs
)

var errBadImage = errors.New("user: unsupported or invalid image")

// processAvatar validates data as a JPEG, PNG, GIF or WebP image, then crops it
// to a square, resizes it to avatarSize and encodes it as WebP (the only format
// the object store accepts). Re-encoding also strips metadata and any payload
// hidden in the original file.
func processAvatar(data []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errBadImage
	}
	if format != "jpeg" && format != "png" && format != "gif" && format != "webp" {
		return nil, errBadImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxSourcePx || cfg.Height > maxSourcePx ||
		cfg.Width*cfg.Height > maxSourceArea {
		return nil, errBadImage
	}

	src, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, errBadImage
	}
	thumb := imaging.Fill(src, avatarSize, avatarSize, imaging.Center, imaging.Lanczos)

	var out bytes.Buffer
	if err := webp.Encode(&out, thumb, webp.Options{Quality: webpQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
