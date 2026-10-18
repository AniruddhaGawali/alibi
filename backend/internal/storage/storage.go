// Package storage stores images in an object store behind the Provider
// interface, so handlers do not care whether the bucket is Supabase Storage,
// S3 or Google Cloud Storage. Every object must be a WebP image.
package storage

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
)

var (
	// ErrNotWebP is returned when the data is not a WebP image. Convert images to
	// WebP before uploading; nothing else is accepted.
	ErrNotWebP = errors.New("storage: only WebP images can be uploaded")
	// ErrInvalidPath is returned for an object path that is empty, absolute,
	// contains "..", unexpected characters, or does not end in ".webp".
	ErrInvalidPath = errors.New("storage: invalid object path")
)

// Provider is an image bucket. Object paths are relative and slash-separated,
// e.g. "avatars/<user_id>_<uuid>.webp" or "cases/case-01/thumbnail.webp".
type Provider interface {
	// UploadImage stores a WebP image at path, replacing any object already
	// there, and returns its public URL.
	UploadImage(ctx context.Context, path string, data []byte) (string, error)
	// Delete removes the object at path. Deleting a missing object is not an error.
	Delete(ctx context.Context, path string) error
	// PublicURL is the URL an object at path is served from.
	PublicURL(path string) string
	// Ping checks the bucket is reachable with the configured credentials.
	Ping(ctx context.Context) error
}

var validPath = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_./-]*\.webp$`)

// CheckPath validates an object path.
func CheckPath(path string) error {
	if !validPath.MatchString(path) || strings.Contains(path, "..") || strings.Contains(path, "//") {
		return ErrInvalidPath
	}
	return nil
}

// IsWebP reports whether data starts with a WebP file header ("RIFF" size "WEBP").
func IsWebP(data []byte) bool {
	return len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
}

// check validates an upload before it leaves the process.
func check(path string, data []byte) error {
	if err := CheckPath(path); err != nil {
		return err
	}
	if !IsWebP(data) {
		return ErrNotWebP
	}
	return nil
}
