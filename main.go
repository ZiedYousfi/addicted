package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type PackageJSONRaw struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type DependencieJSON struct {
	Name    string
	Version string
}

type PackageJSONDependencies struct {
	Dependencies    []DependencieJSON
	DevDependencies []DependencieJSON
}

func mapToDeps(m map[string]string) []DependencieJSON {
	deps := make([]DependencieJSON, 0, len(m))
	for name, version := range m {
		deps = append(deps, DependencieJSON{Name: name, Version: version})
	}
	return deps
}

func getNPMPackageLatestVersion(packageName string) (string, error) {
	url := "https://registry.npmjs.org/" + packageName + "/latest"

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("erreur requête : %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status inattendu : %d", resp.StatusCode)
	}

	var result struct {
		Version string `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("erreur décodage JSON : %w", err)
	}

	return result.Version, nil
}

func main() {
	fmt.Println("Hello, World!")

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Looking for package.json in : %s\n", dir)

	entries, err := os.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	packagePath := ""

	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() == "package.json" {
			packagePath = entry.Name()
			break
		}
	}

	if packagePath == "" {
		log.Panic("No package.json file found in the current directory.")
	}

	fmt.Printf("Found package.json : %s\n", packagePath)

	packageJSONFile, err := os.ReadFile(packagePath)
	if err != nil {
		log.Fatal(err)
	}

	var raw PackageJSONRaw
	if err = json.Unmarshal(packageJSONFile, &raw); err != nil {
		log.Fatal(err)
	}

	var packageJSONDocument map[string]any
	if err = json.Unmarshal(packageJSONFile, &packageJSONDocument); err != nil {
		log.Fatal(err)
	}

	packageJSON := PackageJSONDependencies{
		Dependencies:    mapToDeps(raw.Dependencies),
		DevDependencies: mapToDeps(raw.DevDependencies),
	}

	for i, dep := range packageJSON.Dependencies {
		if strings.Contains(dep.Version, "^") {
			dep.Version, _ = strings.CutPrefix(dep.Version, "^")
			packageJSON.Dependencies[i] = dep
		}
	}

	for i, dep := range packageJSON.DevDependencies {
		if strings.Contains(dep.Version, "^") {
			dep.Version, _ = strings.CutPrefix(dep.Version, "^")
			packageJSON.DevDependencies[i] = dep
		}
	}

	fmt.Printf("Dependencies number : %v\n", len(packageJSON.Dependencies))
	for i, dep := range packageJSON.Dependencies {
		fmt.Printf("Dependency : %s, version : %s\n", dep.Name, dep.Version)

		latestVersion, err := getNPMPackageLatestVersion(dep.Name)
		if err != nil {
			fmt.Printf("Error fetching latest version for %s: %v\n", dep.Name, err)
			continue
		}

		fmt.Printf("Latest version of %s : %s\n", dep.Name, latestVersion)

		dep.Version = latestVersion
		packageJSON.Dependencies[i] = dep
	}

	fmt.Printf("DevDependencies number : %v\n", len(packageJSON.DevDependencies))
	for i, dep := range packageJSON.DevDependencies {
		fmt.Printf("Dependency : %s, version : %s\n", dep.Name, dep.Version)
		if dep.Name == "" {
			fmt.Printf("Dependency name is nil, skipping...\n")
			continue
		}
		latestVersion, err := getNPMPackageLatestVersion(dep.Name)
		if err != nil {
			fmt.Printf("Error fetching latest version for %s: %v\n", dep.Name, err)
			continue
		}

		dep.Version = latestVersion
		packageJSON.DevDependencies[i] = dep

		fmt.Printf("Latest version of %s : %s\n", dep.Name, latestVersion)
	}

	dependenciesMap := make(map[string]string)
	for _, dep := range packageJSON.Dependencies {
		dependenciesMap[dep.Name] = dep.Version
	}

	devDependenciesMap := make(map[string]string)
	for _, dep := range packageJSON.DevDependencies {
		devDependenciesMap[dep.Name] = dep.Version
	}

	if raw.Dependencies != nil || packageJSONDocument["dependencies"] != nil {
		packageJSONDocument["dependencies"] = dependenciesMap
	}

	if raw.DevDependencies != nil || packageJSONDocument["devDependencies"] != nil {
		packageJSONDocument["devDependencies"] = devDependenciesMap
	}

	finalJSON, err := json.MarshalIndent(packageJSONDocument, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(packagePath, append(finalJSON, '\n'), 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Updated %s with final JSON\n", packagePath)
}
