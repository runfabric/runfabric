package artifacts

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ZipResult struct {
	OutputPath string
	SHA256     string
	SizeBytes  int64
}

func ZipSingleFile(sourcePath, outputPath, archiveName string) (*ZipResult, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)

	src, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	w, err := zipWriter.Create(archiveName)
	if err != nil {
		return nil, fmt.Errorf("create zip entry: %w", err)
	}

	if _, err := io.Copy(w, src); err != nil {
		return nil, fmt.Errorf("copy to zip: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("stat zip: %w", err)
	}

	hash, err := fileSHA256(outputPath)
	if err != nil {
		return nil, fmt.Errorf("hash zip: %w", err)
	}

	return &ZipResult{
		OutputPath: outputPath,
		SHA256:     hash,
		SizeBytes:  info.Size(),
	}, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
