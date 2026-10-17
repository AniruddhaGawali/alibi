package user

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // register decoders for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"

	"github.com/disintegration/imaging"
)

const (
	avatarSize    = 256      // processed avatars are avatarSize x avatarSize
	jpegQuality   = 80       // balance of size and fidelity for a small avatar
	maxSourcePx   = 8000     // longest accepted side before decoding
	maxSourceArea = 25 << 20 // pixels; guards against decompression bombs
)

var errBadImage = errors.New("user: unsupported or invalid image")

// processAvatar validates data as a JPEG, PNG or GIF, then crops it to a
// square, resizes it to avatarSize and re-encodes it as JPEG. Re-encoding also
// strips metadata and any payload hidden in the original file.
func processAvatar(data []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errBadImage
	}
	if format != "jpeg" && format != "png" && format != "gif" {
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

	// JPEG has no alpha: flatten transparency onto white instead of black.
	flat := image.NewRGBA(thumb.Bounds())
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), thumb, thumb.Bounds().Min, draw.Over)

	var out bytes.Buffer
	if err := imaging.Encode(&out, flat, imaging.JPEG, imaging.JPEGQuality(jpegQuality)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
