package futils

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/eunanhardy/nori/internal/paths"
	"github.com/eunanhardy/nori/internal/spec"
)

func WriteBlob(data []byte, tag *spec.Tag, mediaType string) (*spec.Digest, error){
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

	fmt.Printf("Writing: sha256:%s\n",hash)
	return &spec.Digest{MediaType: mediaType,Size: fileSize, Digest: "sha256:"+hash},nil
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
    tw := tar.NewWriter(&buf)

    err := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        header, err := tar.FileInfoHeader(fi, fi.Name())
        if err != nil {
            return err
        }

        relativePath, err := filepath.Rel(src, file)
        if err != nil {
            return err
        }

        header.Name = filepath.Join(name, relativePath)

        if err := tw.WriteHeader(header); err != nil {
            return err
        }

        if !fi.Mode().IsDir() {
            data, err := os.Open(file)
            if err != nil {
                return err
            }
            defer data.Close()

            if _, err := io.Copy(tw, data); err != nil {
                return err
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

    return buf.Bytes(), nil
}


func DecompressModule(tarBytes []byte, destPath string) error {
	// Create a reader for the byte slice
	byteReader := bytes.NewReader(tarBytes)
	tarReader := tar.NewReader(byteReader)

	// Iterate over the tar file entries
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of tar archive
		}
		if err != nil {
			return fmt.Errorf("error reading tar archive: %w", err)
		}

		// Create the full path for the file or directory
		target := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory if it doesn't exist
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}
		case tar.TypeReg:
			// Create file
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("error creating file: %w", err)
			}
			// Copy file content
			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("error writing file content: %w", err)
			}
			file.Close()
		default:
			// Handle other file types as needed
			return fmt.Errorf("unsupported file type: %v", header.Typeflag)
		}
	}

	return nil
}