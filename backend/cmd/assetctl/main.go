// Command assetctl uploads images to the object store (Supabase Storage) by
// hand until the admin panel does it:
//
//	assetctl upload <file.webp> <object-path>     one image, e.g. cases/case-01/thumbnail.webp
//	assetctl upload-dir <dir> [prefix]            every image under dir, keeping its relative path
//	assetctl url <object-path>                    print the public URL of an object
//
// Only WebP images are accepted. upload-dir checks every file first and
// uploads nothing if any of them is not a WebP image.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/config"
	"backend/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "assetctl:", err)
		os.Exit(1)
	}
}

func usage() error {
	return fmt.Errorf("usage: assetctl upload <file.webp> <object-path> | upload-dir <dir> [prefix] | url <object-path>")
}

func run(args []string) error {
	if len(args) < 2 {
		return usage()
	}
	cfg, err := config.LoadAssets()
	if err != nil {
		return err
	}
	p := storage.NewSupabaseProvider(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey, cfg.SupabaseBucket)

	switch args[0] {
	case "url":
		if err := storage.CheckPath(args[1]); err != nil {
			return err
		}
		fmt.Println(p.PublicURL(args[1]))
		return nil
	case "upload":
		if len(args) != 3 {
			return usage()
		}
		return upload(p, []file{{local: args[1], object: args[2]}})
	case "upload-dir":
		prefix := ""
		if len(args) == 3 {
			prefix = strings.Trim(args[2], "/")
		}
		files, err := collect(args[1], prefix)
		if err != nil {
			return err
		}
		return upload(p, files)
	}
	return usage()
}

type file struct{ local, object string }

// collect lists the images under dir (skipping dotfiles and README/text files).
func collect(dir, prefix string) ([]file, error) {
	var out []file
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && p != dir {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || strings.EqualFold(filepath.Ext(p), ".md") {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out = append(out, file{local: p, object: path.Join(prefix, filepath.ToSlash(rel))})
		return nil
	})
	return out, err
}

// upload validates every file, then uploads them all.
func upload(p storage.Provider, files []file) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to upload")
	}
	data := make([][]byte, len(files))
	var bad []string
	for i, f := range files {
		b, err := os.ReadFile(f.local)
		if err != nil {
			return err
		}
		switch {
		case storage.CheckPath(f.object) != nil:
			bad = append(bad, fmt.Sprintf("%s: invalid object path %q (letters, digits, _ - . / only, must end in .webp)", f.local, f.object))
		case !storage.IsWebP(b):
			bad = append(bad, f.local+": not a WebP image (convert it first, e.g. `cwebp -q 82 in.jpg -o out.webp`)")
		}
		data[i] = b
	}
	if len(bad) > 0 {
		return fmt.Errorf("nothing uploaded:\n  %s", strings.Join(bad, "\n  "))
	}
	for i, f := range files {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		url, err := p.UploadImage(ctx, f.object, data[i])
		cancel()
		if err != nil {
			return fmt.Errorf("%s: %w", f.local, err)
		}
		fmt.Printf("%s -> %s\n", f.local, url)
	}
	return nil
}
