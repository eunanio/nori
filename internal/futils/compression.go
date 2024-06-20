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

	"github.com/eunanhardy/nori/internal/spec"
)

func WriteBlob(data []byte, path string, mediaType string) (*spec.Digest, error){
	hasher := sha256.New()
	hasher.Write(data)
	hash := hex.EncodeToString(hasher.Sum(nil))
	fileSize := int64(len(data))
	dstFile, err := os.Create(fmt.Sprintf("%s/%s", path, hash)); if err != nil {
		return nil,err
	}
	defer dstFile.Close()

	dstFile.Write(data)
	return &spec.Digest{MediaType: mediaType,Size: fileSize, Digest: "sha256:"+hash},nil
}

func CompressBytes(data []byte, path string, mediaType string) (*spec.Digest, error) {
	    // Create a bytes buffer from the data
		src := bytes.NewBuffer(data)
		dataSize := int64(len(data))

		hasher := sha256.New()
		hasher.Write(data)
		hash := hex.EncodeToString(hasher.Sum(nil))
		// Create the destination file and defer its closure
		dstFile, err := os.Create(fmt.Sprintf("%s/%s", path, hash))
		if err != nil {
			return nil,err
		}
		defer dstFile.Close()
	
		tw := tar.NewWriter(dstFile)
		defer tw.Close()
	
		header := &tar.Header{
			Name: hash,
			Size: dataSize,
			Mode: 0600, // File permissions
		}
	
		if err := tw.WriteHeader(header); err != nil {
			return nil,err
		}
	
		// Write the data to the tar writer
		if _, err := io.Copy(tw, src); err != nil {
			return nil,err
		}
	
		return &spec.Digest{MediaType: mediaType,Size: dataSize, Digest: "sha256:"+hash},nil
}

func CompressDir(src string, tarPath string, mediaType string, name string) (*spec.Digest, error) {
	// Open the source directory and defer its closure
	dst := tarPath+"/module.layer.tar"
	os.Create(dst)
	srcFile, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("failed to open source directory: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	tw := tar.NewWriter(dstFile)
	//defer tw.Close()

	err = filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(src, file); if err != nil {
			return err
		}

		header.Name = filepath.Join(name, relativePath)

		// Write the header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	tw.Close()

	if _, err := dstFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek destination file: %w", err)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, dstFile); err != nil {
		return nil, fmt.Errorf("failed to compute SHA-256 checksum: %w", err)
	}
	shaHash := hex.EncodeToString(hasher.Sum(nil))

	fileInfo, err := dstFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	os.Rename(dst, fmt.Sprintf("%s/%s", tarPath, shaHash))
	return &spec.Digest{MediaType: mediaType, Size: fileSize, Digest:"sha256:"+shaHash}, nil
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

func GetFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return fileInfo.Size(), nil
}

func HashZip(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}