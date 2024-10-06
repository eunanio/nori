package futils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/paths"
	"github.com/eunanio/nori/internal/spec"
)

func WriteBlob(data []byte, mediaType string) (*spec.Digest, error) {
	hasher := sha256.New()
	hasher.Write(data)
	hash := hex.EncodeToString(hasher.Sum(nil))
	fileSize := int64(len(data))
	blobDirPath := paths.GetBlobDirV2(hash)
	blobPath := paths.GetBlobPathV2(hash)
	paths.MkDirIfNotExist(blobDirPath)
	err := os.WriteFile(blobPath, data, 0644)
	if err != nil {
		return nil, fmt.Errorf("error writing blob: %s", err)
	}

	console.Debug(fmt.Sprintf("Writing: sha256:%s\n", hash))
	return &spec.Digest{MediaType: mediaType, Size: fileSize, Digest: "sha256:" + hash}, nil
}

func LoadBlob(digest string) (data []byte, err error) {
	sha := digest[7:]
	blobPath := paths.GetBlobPathV2(sha)
	if FileExists(blobPath) {
		blobBytes, err := os.ReadFile(blobPath)
		return blobBytes, err
	}
	return nil, fmt.Errorf("file not found")
}

// CompressDir compresses the given directory and returns the tar file as a byte array.
func CompressModule(src string, name string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	err := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		relativePath, err := filepath.Rel(src, file)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		header.Name = filepath.Join(name, relativePath)
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		if !fi.Mode().IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer data.Close()

			if _, err := io.Copy(tw, data); err != nil {
				return fmt.Errorf("failed to copy file data: %w", err)
			}

		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func DecompressModule(tarBytes []byte, destPath string) error {
	byteReader := bytes.NewReader(tarBytes)
	gzipReader, err := gzip.NewReader(byteReader)
	if err != nil {
		return fmt.Errorf("error creating gzip reader: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of tar archive
		}
		if err != nil {
			return fmt.Errorf("error reading tar archive: %w", err)
		}

		target := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}
		case tar.TypeReg:
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("error creating file: %w", err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("error writing file content: %w", err)
			}
			file.Close()
		default:
			return fmt.Errorf("unsupported file type: %v", header.Typeflag)
		}
	}

	return nil
}
