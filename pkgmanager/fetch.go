package pkgmanager

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"

	"github.com/Masterminds/semver/v3"
)

// PackageInfo represents the structure of the package info returned by the NPM registry
type PackageInfo struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Dist    map[string]interface{} `json:"dist"`
}

// FetchPackageInfo fetches package information from the NPM registry
func FetchPackageInfo(packageName, version string) (*PackageInfo, error) {
	encodedPackageName := url.PathEscape(packageName)
	registryURL := fmt.Sprintf("https://registry.npmjs.org/%s", encodedPackageName)
	resp, err := http.Get(registryURL)
	if err != nil {
		log.Printf("failed to fetch package info: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("failed to fetch package info: %v", resp.Status)
		return nil, fmt.Errorf("failed to fetch package info: %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		return nil, err
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(body, &metadata); err != nil {
		log.Printf("failed to unmarshal JSON: %v", err)
		return nil, err
	}

	// Resolve the version range to a specific version
	resolvedVersion, err := resolveVersion(metadata, version)
	if err != nil {
		log.Printf("failed to resolve version: %v", err)
		return nil, err
	}

	// Fetch the specific version info
	packageInfo := &PackageInfo{}
	if v, ok := metadata["versions"].(map[string]interface{})[resolvedVersion]; ok {
		packageInfoJSON, err := json.Marshal(v)
		if err != nil {
			log.Printf("failed to marshal package info: %v", err)
			return nil, err
		}
		if err := json.Unmarshal(packageInfoJSON, packageInfo); err != nil {
			log.Printf("failed to unmarshal package info: %v", err)
			return nil, err
		}
	}

	return packageInfo, nil
}

// resolveVersion resolves a version range to a specific version
func resolveVersion(metadata map[string]interface{}, versionRange string) (string, error) {
	if versionRange == "latest" {
		if distTags, ok := metadata["dist-tags"].(map[string]interface{}); ok {
			if latest, ok := distTags["latest"].(string); ok {
				return latest, nil
			}
		}
	}

	versions := []string{}
	if versionMap, ok := metadata["versions"].(map[string]interface{}); ok {
		for v := range versionMap {
			versions = append(versions, v)
		}
	}

	// Sort versions using semver
	sort.Slice(versions, func(i, j int) bool {
		vi, err := semver.NewVersion(versions[i])
		if err != nil {
			return false
		}
		vj, err := semver.NewVersion(versions[j])
		if err != nil {
			return false
		}
		return vi.GreaterThan(vj)
	})

	// Match version range
	constraint, err := semver.NewConstraint(versionRange)
	if err != nil {
		return "", fmt.Errorf("invalid version range: %s", versionRange)
	}

	for _, v := range versions {
		ver, err := semver.NewVersion(v)
		if err != nil {
			continue
		}
		if constraint.Check(ver) {
			return v, nil
		}
	}

	return "", fmt.Errorf("no matching version found for range: %s", versionRange)
}
