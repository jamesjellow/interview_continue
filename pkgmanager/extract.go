package pkgmanager

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ExtractTarball extracts a tarball to a directory named after the package within the specified destination directory
func ExtractTarball(tarballPath, destDir, packageName string) error {
	// Create the package directory with just the package name
	packageDir := filepath.Join(destDir, packageName)
	if err := os.MkdirAll(packageDir, os.ModePerm); err != nil {
		log.Printf("failed to create package directory: %v", err)
		return err
	}

	file, err := os.Open(tarballPath)
	if err != nil {
		log.Printf("failed to open tarball: %v", err)
		return err
	}
	defer file.Close()

	// Defer the cleanup of the tarball file
	defer func() {
		if err := os.Remove(tarballPath); err != nil {
			log.Printf("failed to remove tarball: %v", err)
		}
	}()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		log.Printf("failed to create gzip reader: %v", err)
		return err
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("failed to read tarball: %v", err)
			return err
		}

		// Skip the initial 'package' directory
		header.Name = strings.TrimPrefix(header.Name, "package/")

		path := filepath.Join(packageDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				log.Printf("failed to create directory: %v", err)
				return err
			}
		case tar.TypeReg:
			// Ensure the directory exists
			if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
				log.Printf("failed to create directory: %v", err)
				return err
			}

			outFile, err := os.Create(path)
			if err != nil {
				log.Printf("failed to create file: %v", err)
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				log.Printf("failed to copy file: %v", err)
				return err
			}
			outFile.Close()
		default:
			log.Printf("unsupported tar header type: %v", header.Typeflag)
			return fmt.Errorf("unsupported tar header type: %v", header.Typeflag)
		}
	}

	return nil
}
