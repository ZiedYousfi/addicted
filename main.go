package main

import (
	"encoding/json"
	"fmt"
	"log"
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

	packageJSONFile, err := os.Open(packagePath)
	if err != nil {
		log.Fatal(err)
	}
	defer packageJSONFile.Close()

	var raw PackageJSONRaw
	if err = json.NewDecoder(packageJSONFile).Decode(&raw); err != nil {
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
	for _, dep := range packageJSON.Dependencies {
		fmt.Printf("Dependency : %s, version : %s\n", dep.Name, dep.Version)
	}

	fmt.Printf("DevDependencies number : %v\n", len(packageJSON.DevDependencies))
	for _, dep := range packageJSON.DevDependencies {
		fmt.Printf("Dependency : %s, version : %s\n", dep.Name, dep.Version)
	}
}
