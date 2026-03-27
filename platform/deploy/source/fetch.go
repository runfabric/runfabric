package source

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FetchAndExtract downloads the archive at url (http/https), extracts it to a temporary directory,
// and returns the path to the extracted root and a cleanup function. Caller must call cleanup() when done.
// Supports .zip and .tar.gz/.tgz. Looks for runfabric.yml in the extracted tree (root or one level down).
func FetchAndExtract(url string) (extractRoot string, configPath string, cleanup func(), err error) {
	cleanup = func() {}
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", cleanup, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", cleanup, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	dir, err := os.MkdirTemp("", "runfabric-source-*")
	if err != nil {
		return "", "", cleanup, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	ctype := resp.Header.Get("Content-Type")
	body := resp.Body
	if resp.ContentLength > 0 {
		body = io.NopCloser(io.LimitReader(resp.Body, resp.ContentLength+1024))
	}

	switch {
	case strings.HasSuffix(strings.ToLower(url), ".zip"), strings.Contains(ctype, "zip"):
		if err := extractZip(body, dir); err != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("extract zip: %w", err)
		}
	default:
		// .tar.gz, .tgz or default
		gz, err := gzip.NewReader(body)
		if err != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		if err := extractTar(gz, dir); err != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("extract tar: %w", err)
		}
	}

	configPath, err = findRunfabricYAML(dir)
	if err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	extractRoot = filepath.Dir(configPath)
	return extractRoot, configPath, cleanup, nil
}

func extractZip(r io.Reader, dest string) error {
	tmpZip, err := os.CreateTemp("", "runfabric-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpZip.Name())
	if _, err := io.Copy(tmpZip, r); err != nil {
		return err
	}
	if err := tmpZip.Close(); err != nil {
		return err
	}
	zr, err := zip.OpenReader(tmpZip.Name())
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Join(dest, filepath.Clean(f.Name))
		if !strings.HasPrefix(name, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(name)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(w, rc)
		rc.Close()
		w.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Join(dest, filepath.Clean(h.Name))
		if !strings.HasPrefix(name, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		if h.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(name, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		w, err := os.Create(name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, tr); err != nil {
			w.Close()
			return err
		}
		w.Close()
	}
	return nil
}

func findRunfabricYAML(dir string) (string, error) {
	var found string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base == "runfabric.yml" || base == "runfabric.yaml" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no runfabric.yml found in archive")
	}
	return found, nil
}
