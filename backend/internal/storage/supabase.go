package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// cacheSeconds is the Cache-Control max-age set on uploaded objects. Object
// names are unique per upload (avatars get a fresh UUID), so they never change.
const cacheSeconds = 31536000

// SupabaseProvider stores images in a public Supabase Storage bucket through
// its REST API, authenticated with the service-role key (which bypasses
// row-level security, so it must only ever live on the server).
type SupabaseProvider struct {
	baseURL string // https://<project>.supabase.co
	key     string
	bucket  string
	client  *http.Client
}

// NewSupabaseProvider returns a provider for bucket in the project at baseURL.
func NewSupabaseProvider(baseURL, serviceRoleKey, bucket string) *SupabaseProvider {
	return &SupabaseProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     serviceRoleKey,
		bucket:  bucket,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

var _ Provider = (*SupabaseProvider)(nil)

func (p *SupabaseProvider) objectURL(path string) string {
	return fmt.Sprintf("%s/storage/v1/object/%s/%s", p.baseURL, url.PathEscape(p.bucket), escapePath(path))
}

// PublicURL is <url>/storage/v1/object/public/<bucket>/<path>.
func (p *SupabaseProvider) PublicURL(path string) string {
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", p.baseURL, url.PathEscape(p.bucket), escapePath(path))
}

func (p *SupabaseProvider) UploadImage(ctx context.Context, path string, data []byte) (string, error) {
	if err := check(path, data); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.objectURL(path), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	p.auth(req)
	req.Header.Set("Content-Type", "image/webp")
	req.Header.Set("Cache-Control", fmt.Sprintf("max-age=%d", cacheSeconds))
	req.Header.Set("x-upsert", "true")
	if err := p.do(req, "upload "+path); err != nil {
		return "", err
	}
	return p.PublicURL(path), nil
}

func (p *SupabaseProvider) Delete(ctx context.Context, path string) error {
	if err := CheckPath(path); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.objectURL(path), nil)
	if err != nil {
		return err
	}
	p.auth(req)
	err = p.do(req, "delete "+path)
	var se *StatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return nil
	}
	return err
}

func (p *SupabaseProvider) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/storage/v1/bucket/%s", p.baseURL, url.PathEscape(p.bucket)), nil)
	if err != nil {
		return err
	}
	p.auth(req)
	return p.do(req, "get bucket")
}

func (p *SupabaseProvider) auth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.key)
	req.Header.Set("apikey", p.key)
}

// StatusError is a non-2xx answer from the storage API.
type StatusError struct {
	Op   string
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("storage: %s: status %d: %s", e.Op, e.Code, e.Body)
}

func (p *SupabaseProvider) do(req *http.Request, op string) error {
	res, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("storage: %s: %w", op, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		// Supabase reports a missing object as 400 with statusCode "404" in the body.
		code := res.StatusCode
		if code == http.StatusBadRequest && bytes.Contains(body, []byte(`"404"`)) {
			code = http.StatusNotFound
		}
		return &StatusError{Op: op, Code: code, Body: strings.TrimSpace(string(body))}
	}
	return nil
}

// escapePath escapes each segment of a slash-separated path.
func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
