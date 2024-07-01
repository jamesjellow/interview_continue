package pkgmanager

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadPackage downloads the package tarball from the given URL and verifies the checksum
func DownloadPackage(tarballURL, expectedShasum, destDir string) (string, error) {
	resp, err := http.Get(tarballURL)
	if err != nil {
		log.Printf("failed to download package: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("failed to download package: %v", resp.Status)
		return "", fmt.Errorf("failed to download package: %v", resp.Status)
	}

	fileName := filepath.Base(tarballURL)
	destPath := filepath.Join(destDir, fileName)
	out, err := os.Create(destPath)
	if err != nil {
		log.Printf("failed to create file: %v", err)
		return "", err
	}
	defer out.Close()

	hasher := sha1.New()
	tee := io.TeeReader(resp.Body, hasher)

	_, err = io.Copy(out, tee)
	if err != nil {
		log.Printf("failed to copy file: %v", err)
		return "", err
	}

	calculatedShasum := fmt.Sprintf("%x", hasher.Sum(nil))
	if calculatedShasum != expectedShasum {
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", expectedShasum, calculatedShasum)
	}

	return destPath, nil
}
