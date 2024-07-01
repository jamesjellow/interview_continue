package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/dominikbraun/graph"
	"github.com/iancoleman/orderedmap"
	"github.com/jamesjellow/fpm/pkgmanager"
)

var (
	NodeModulesDir = "./node_modules"
)

// Runner for handlers to install a package
func RunInstallPackage(packageName string, packageVersion string, depGraph *graph.Graph[string, string], forDevDependency bool) (string, error) {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Installing %s@%s", packageName, packageVersion)
	s.Start()
	defer s.Stop()

	actualVersion, err := installPackage(packageName, packageVersion, depGraph)
	if err != nil {
		return actualVersion, err
	}
	finishedStr := fmt.Sprintf("Finished %v@%v", packageName, packageVersion)
	if forDevDependency {
		finishedStr += " (for dev dependency)"
	}
	s.Stop()
	fmt.Println(finishedStr)

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
func installPackage(packageName string, packageVersion string, depGraph *graph.Graph[string, string]) (string, error) {
	// Check if the package and all its dependencies are already installed, if so then skip it
	visited := make(map[string]bool)
	if isPackageAndDependenciesInNodeModules(packageName, visited) {
		if err := (*depGraph).AddVertex(packageName); err != nil && err != graph.ErrVertexAlreadyExists {
			return "", fmt.Errorf("failed to add vertex: %v", err)
		}
		return packageVersion, nil
	}

	// Fetch the package info from the npm registry
	packageInfo, err := pkgmanager.FetchPackageInfo(packageName, packageVersion)
	if err != nil {
		return "", fmt.Errorf("failed to fetch package info: %v", err)
	}

	actualVersion := packageInfo.Version                      // react@latest -> react@18.3.1
	packagePath := filepath.Join(NodeModulesDir, packageName) // ./node_modules/react

	// Download the package
	tarballURL := packageInfo.Dist["tarball"].(string)
	expectedShasum := packageInfo.Dist["shasum"].(string)
	tarballPath, err := pkgmanager.DownloadPackage(tarballURL, expectedShasum, NodeModulesDir)
	if err != nil {
		return "", fmt.Errorf("failed to download package: %v", err)
	}

	// Extract the package
	if err := pkgmanager.ExtractTarball(tarballPath, NodeModulesDir, packageName); err != nil {
		return "", fmt.Errorf("failed to extract package: %v", err)
	}

	// Walk the directories and get all the package.json files for a given package path
	packageJsonPaths, err := findPackageJsonFiles(packagePath)
	if err != nil {
		return "", fmt.Errorf("failed to find package.json files: %v", err)
	}

	// Add the package to the dependency graph
	if err := (*depGraph).AddVertex(packageName); err != nil && err != graph.ErrVertexAlreadyExists {
		return "", fmt.Errorf("failed to add vertex: %v", err)
	}

	// For each of the package jsons, install their dependencies
	for _, packageJsonPath := range packageJsonPaths {
		content, err := os.ReadFile(packageJsonPath)
		if err != nil {
			return "", fmt.Errorf("failed to read package.json: %v", err)
		}

		var packageJson map[string]interface{}
		if err := json.Unmarshal(content, &packageJson); err != nil {
			return "", fmt.Errorf("failed to decode package.json: %v", err)
		}

		dependencies, ok := packageJson["dependencies"].(map[string]interface{})
		if !ok {
			// No dependencies, this is not an error
			return "", nil
		}

		for depName, depVersion := range dependencies {
			versionStr, ok := depVersion.(string)
			if !ok {
				return "", fmt.Errorf("invalid version for dependency %s: %v", depName, depVersion)
			}

			if err := (*depGraph).AddVertex(depName); err != nil && err != graph.ErrVertexAlreadyExists {
				return "", fmt.Errorf("failed to add vertex: %v", err)
			}
			if err := (*depGraph).AddEdge(packageName, depName); err != nil && err != graph.ErrEdgeAlreadyExists {
				return "", fmt.Errorf("failed to add edge: %v", err)
			}

			// Recursively call installPackage with the additional deps we need to install
			if _, err := installPackage(depName, versionStr, depGraph); err != nil {
				return "", err
			}
		}

	}

	return "^" + actualVersion, nil
}

// Check if the package exists in the node_modules directory
func isPackageAndDependenciesInNodeModules(packageName string, visited map[string]bool) bool {
	if visited[packageName] {
		return true
	}
	visited[packageName] = true

	var packagePath string

	// Handle scoped packages
	if strings.HasPrefix(packageName, "@") {
		parts := strings.Split(packageName, "/")
		if len(parts) != 2 {
			return false
		}
		packagePath = filepath.Join(NodeModulesDir, parts[0], parts[1])
	} else {
		packagePath = filepath.Join(NodeModulesDir, packageName)
	}

	// Check if the package directory exists
	if _, err := os.Stat(packagePath); err != nil {
		return false
	}

	// Read and parse package.json
	packageJsonPath := filepath.Join(packagePath, "package.json")
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return false
	}

	var packageJson map[string]interface{}
	if err := json.Unmarshal(content, &packageJson); err != nil {
		return false
	}

	// Check dependencies
	dependencies, ok := packageJson["dependencies"].(map[string]interface{})
	if ok {
		for dep := range dependencies {
			if !isPackageAndDependenciesInNodeModules(dep, visited) {
				return false
			}
		}
	}

	return true
}

// Walk the folders to get all the package.json files
func findPackageJsonFiles(dir string) ([]string, error) {
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == "node_modules" && path != dir {
			return filepath.SkipDir
		}
		if !info.IsDir() && info.Name() == "package.json" {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
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

func RemoveTarballs(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".tgz") {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
}
