package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/dominikbraun/graph"
	"github.com/iancoleman/orderedmap"
	"github.com/jamesjellow/fpm/pkgmanager"
)

var (
	NodeModulesDir     = "./node_modules"
	installingPackages = make(map[string]bool)
	installMutex       sync.Mutex
)

// Runner for handlers to install a package
func RunInstallPackage(packageName string, packageVersion string, depGraph *graph.Graph[string, string], forDevDependency bool) (string, error) {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Installing %s@%s", packageName, packageVersion)
	s.Start()
	defer s.Stop()

	visited := make(map[string]bool)
	actualVersion, err := installPackage(packageName, packageVersion, depGraph, visited)
	if err != nil {
		return actualVersion, err
	}

	s.Stop()
	fmt.Printf("✔ Installed %s@%s\n", packageName, actualVersion)

	return actualVersion, nil
}

// Read a package.json file and returns its contents as an ordered map
func ParsePackageJson(pathToJSON string) (*orderedmap.OrderedMap, error) {
	file, err := os.Open(pathToJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	result := orderedmap.New()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(result); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %v", err)
	}

	return result, nil
}

// Parse a package argument and returns the name of the package and its version example: react@latest
func ParsePackageArg(arg string) (string, string) {
	if strings.HasPrefix(arg, "@") {
		parts := strings.SplitN(arg, "@", 3)
		if len(parts) == 3 {
			return "@" + parts[1], parts[2]
		}
		return arg, "latest"
	}

	parts := strings.SplitN(arg, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return arg, "latest"
}

// Logic for installing a package and keeping track of known deps in a graph.
func installPackage(packageName string, packageVersion string, depGraph *graph.Graph[string, string], visited map[string]bool) (string, error) {
	installMutex.Lock()
	if installingPackages[packageName] {
		installMutex.Unlock()
		return packageVersion, nil // Already being installed, avoid cycles
	}
	installingPackages[packageName] = true
	installMutex.Unlock()

	// Remember to cleanup after done installing
	defer func() {
		installMutex.Lock()
		delete(installingPackages, packageName)
		installMutex.Unlock()
	}()

	if visited[packageName] {
		return packageVersion, nil // Already visited, avoid cycles
	}
	visited[packageName] = true

	// Check if the package is installed, if so add a vertex to the dep graph
	packagePath := filepath.Join(NodeModulesDir, packageName)
	if strings.HasPrefix(packageName, "@") {
		parts := strings.SplitN(packageName, "/", 2)
		if len(parts) == 2 {
			packagePath = filepath.Join(NodeModulesDir, parts[0], parts[1])
		}
	}
	_, err := os.Stat(packagePath)
	if err == nil {
		if err := (*depGraph).AddVertex(packageName); err != nil && err != graph.ErrVertexAlreadyExists {
			return "", fmt.Errorf("failed to add vertex: %v", err)
		}
		return packageVersion, nil
	}

	// Get the package info from the registry
	packageInfo, err := pkgmanager.FetchPackageInfo(packageName, packageVersion)
	if err != nil {
		return "", fmt.Errorf("failed to fetch package info: %v", err)
	}
	actualVersion := packageInfo.Version

	// Download
	tarballURL := packageInfo.Dist["tarball"].(string)
	expectedShasum := packageInfo.Dist["shasum"].(string)
	tarballPath, err := pkgmanager.DownloadPackage(tarballURL, expectedShasum, NodeModulesDir)
	if err != nil {
		return "", fmt.Errorf("failed to download package: %v", err)
	}

	// Extract
	extractDir := NodeModulesDir
	if strings.HasPrefix(packageName, "@") {
		parts := strings.SplitN(packageName, "/", 2)
		if len(parts) == 2 {
			extractDir = filepath.Join(NodeModulesDir, parts[0])
		}
	}
	if err := pkgmanager.ExtractTarball(tarballPath, extractDir, packageName); err != nil {
		return "", fmt.Errorf("failed to extract package: %v", err)
	}

	// Add to dep graph
	if err := (*depGraph).AddVertex(packageName); err != nil && err != graph.ErrVertexAlreadyExists {
		return "", fmt.Errorf("failed to add vertex: %v", err)
	}

	// Find the first package JSON
	packageJsonPath, err := findPackageJson(packageName)
	if err != nil {
		log.Printf("Warning: %v, skipping dependency installation", err)
		return actualVersion, nil
	}

	// Process the main package.json
	if err := processPackageJson(packageJsonPath, packageName, depGraph, visited); err != nil {
		return "", err
	}

	// Check for additional package.json files
	packageDir := filepath.Dir(packageJsonPath)
	additionalPackageJsons, err := findAdditionalPackageJsons(packageDir)
	if err != nil {
		log.Printf("Warning: Error finding additional package.json files: %v", err)
	} else {
		for _, additionalPath := range additionalPackageJsons {
			if err := processPackageJson(additionalPath, packageName, depGraph, visited); err != nil {
				log.Printf("Warning: Error processing additional package.json at %s: %v", additionalPath, err)
			}
		}
	}

	return actualVersion, nil
}

// As the name implies, get all the deps from the package.json file and return a map of them
func getDependenciesFromPackageJson(packageJsonPath string) (map[string]string, error) {
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %v", err)
	}

	var packageJson map[string]interface{}
	if err := json.Unmarshal(content, &packageJson); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %v", err)
	}

	dependencies := make(map[string]string)
	if deps, ok := packageJson["dependencies"].(map[string]interface{}); ok {
		for name, version := range deps {
			dependencies[name] = version.(string)
		}
	}

	return dependencies, nil
}

// Find the package json for the given package name and return the path to its package json file
func findPackageJson(packageName string) (string, error) {
	possiblePaths := []string{
		filepath.Join(NodeModulesDir, packageName, "package.json"),
		filepath.Join(NodeModulesDir, strings.Replace(packageName, "/", "/@", 1), "package.json"),
		filepath.Join(NodeModulesDir, strings.Replace(packageName, "/", "/@", 1), packageName, "package.json"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// If not found in predefined paths, do a recursive search
	var packageJsonPath string
	err := filepath.Walk(NodeModulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "package.json" && strings.Contains(path, packageName) {
			packageJsonPath = path
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	if packageJsonPath == "" {
		return "", fmt.Errorf("package.json not found for %s", packageName)
	}
	return packageJsonPath, nil
}

// Write to the packageJson with the new dependencies that you are adding
func UpdatePackageJson(pathToJSON string, newDependencies map[string]string, forDev bool) error {
	dependencyKey := "dependencies"
	if forDev {
		dependencyKey = "devDependencies"
	}

	packageJson, err := ParsePackageJson(pathToJSON)
	if err != nil {
		return err
	}

	depsMap, err := ParseDependencies(packageJson, dependencyKey)
	if err != nil {
		return err
	}

	for name, version := range newDependencies {
		depsMap.Set(name, version)
	}

	sortedDeps := orderedmap.New()
	keys := depsMap.Keys()
	sort.Strings(keys)
	for _, key := range keys {
		value, _ := depsMap.Get(key)
		sortedDeps.Set(key, value)
	}

	packageJson.Set(dependencyKey, sortedDeps)

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "    ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(packageJson); err != nil {
		return fmt.Errorf("failed to encode package.json: %v", err)
	}

	data := bytes.ReplaceAll(buffer.Bytes(), []byte("\\u0026"), []byte("&"))

	err = os.WriteFile(pathToJSON, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write package.json: %v", err)
	}

	return nil
}

// Get the (dependency, version) returned as an ordered map
func ParseDependencies(packageJson *orderedmap.OrderedMap, dependencyType string) (*orderedmap.OrderedMap, error) {
	deps, ok := packageJson.Get(dependencyType)
	if !ok {
		deps = orderedmap.New()
		packageJson.Set(dependencyType, deps)
	}

	var depsMap *orderedmap.OrderedMap

	switch v := deps.(type) {
	case orderedmap.OrderedMap:
		depsMap = &v
	case *orderedmap.OrderedMap:
		depsMap = v
	default:
		return nil, fmt.Errorf("unexpected type for dependencies: %T", v)
	}

	return depsMap, nil
}

// Try to recursively process all the dependencies in the package.json file and add them to the graph
func processPackageJson(packageJsonPath, packageName string, depGraph *graph.Graph[string, string], visited map[string]bool) error {
	dependencies, err := getDependenciesFromPackageJson(packageJsonPath)
	if err != nil {
		return err
	}

	for depName, depVersion := range dependencies {
		if err := (*depGraph).AddVertex(depName); err != nil && err != graph.ErrVertexAlreadyExists {
			log.Printf("Warning: failed to add vertex for %s: %v", depName, err)
			continue
		}

		err := (*depGraph).AddEdge(packageName, depName)
		if err != nil {
			if err == graph.ErrEdgeAlreadyExists {
				// Edge already exists, this is fine, continue
				continue
			}
			if strings.Contains(err.Error(), "cycle") {
				// Circular dependency detected, log a warning and continue
				continue
			}
			// For other errors, log a warning and continue
			log.Printf("\n  - Warning: failed to add edge from %s to %s: %v", packageName, depName, err)
			continue
		}

		if _, err := installPackage(depName, depVersion, depGraph, visited); err != nil {
			// Log the error but continue with other dependencies
			log.Printf("\n  - Error installing dependency %s: %v", depName, err)
		}
	}

	return nil
}

// See if there's any more package jsons in the current directory. Ifso, return them. Otherwise, return an empty array and no error.
func findAdditionalPackageJsons(dir string) ([]string, error) {
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "package.json" && path != filepath.Join(dir, "package.json") {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}
