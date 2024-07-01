package handlers

import (
	"fmt"
	"os"

	"github.com/dominikbraun/graph"
	"github.com/jamesjellow/fpm/utils"
)

var PackageJsonPath = "./package.json"

func HandleAdd(args []string, depGraph *graph.Graph[string, string]) error {
	if len(args) < 3 {
		return fmt.Errorf("expected package name after 'add'")
	}

	// Parse the second arg "package@version"
	packageName, packageVersion := utils.ParsePackageArg(args[2])
	forDevDependency := len(args) == 4 && args[3] == "-D"

	// Ensure package.json exists
	if _, err := os.Stat(PackageJsonPath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found")
	}

	// Ensure the node_modules directory exists
	if err := os.MkdirAll(utils.NodeModulesDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create node_modules directory: %v", err)
	}

	// Install the package
	actualVersion, err := utils.RunInstallPackage(packageName, packageVersion, depGraph, forDevDependency)
	if err != nil {
		return err
	}

	// Update the package.json file with the new dependency
	if err = utils.UpdatePackageJson(PackageJsonPath, map[string]string{packageName: actualVersion}, forDevDependency); err != nil {
		return fmt.Errorf("failed to update package.json: %v", err)

	}

	return nil
}

func HandleInstall(depGraph *graph.Graph[string, string]) error {
	// Get the packageJSON  into a map
	packageJSON, err := utils.ParsePackageJson(PackageJsonPath)
	if err != nil {
		return err
	}

	// Ensure the node_modules directory exists
	if err := os.MkdirAll(utils.NodeModulesDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create node_modules directory: %v", err)
	}

	// Install each dependency
	for _, depType := range []string{"dependencies", "devDependencies"} {
		deps, err := utils.ParseDependencies(packageJSON, depType)
		if err != nil {
			return err
		}

		for _, dep := range deps.Keys() {
			version, ok := deps.Get(dep)
			if !ok {
				return fmt.Errorf("failed to get version for dependency: %s", dep)
			}

			versionStr, ok := version.(string)
			if !ok {
				return fmt.Errorf("version for dependency %s is not a string: %T", dep, version)
			}

			forDevDependency := depType == "devDependencies"
			if _, err := utils.RunInstallPackage(dep, versionStr, depGraph, forDevDependency); err != nil {
				return err
			}

		}
	}

	fmt.Println("✔ All packages installed successfully")
	return nil
}
