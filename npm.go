package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type PackageJSONRaw struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type DependencyJSON struct {
	Name    string
	Version string
}

type PackageJSONDependencies struct {
	Dependencies    []DependencyJSON
	DevDependencies []DependencyJSON
}

func mapToDeps(m map[string]string) []DependencyJSON {
	deps := make([]DependencyJSON, 0, len(m))
	for name, version := range m {
		deps = append(deps, DependencyJSON{Name: name, Version: version})
	}
	return deps
}

func getNPMPackageLatestVersion(packageName string) (string, error) {
	registryURL := "https://registry.npmjs.org/" + url.PathEscape(packageName) + "/latest"
	client := Ctx.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	resp, err := client.Get(registryURL)
	if err != nil {
		return "", fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		Version string `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("JSON decode error: %w", err)
	}

	return result.Version, nil
}

func normalizeDependencyVersions(deps []DependencyJSON) {
	prefixes := []string{"^", "~", ">=", "<=", ">", "<", "=", "*"}

	for i, dep := range deps {
		for _, prefix := range prefixes {
			if normalizedVersion, ok := strings.CutPrefix(dep.Version, prefix); ok {
				dep.Version = normalizedVersion
				deps[i] = dep
				break
			}
		}
	}
}

func updateDependencies(deps []DependencyJSON) {
	for i, dep := range deps {
		fmt.Printf("Dependency : %s, version : %s\n", dep.Name, dep.Version)
		if dep.Name == "" {
			fmt.Printf("Dependency name is empty, skipping...\n")
			continue
		}

		latestVersion, err := getNPMPackageLatestVersion(dep.Name)
		if err != nil {
			fmt.Printf("Error fetching latest version for %s: %v\n", dep.Name, err)
			continue
		}

		fmt.Printf("Latest version of %s : %s\n", dep.Name, latestVersion)
		dep.Version = latestVersion
		deps[i] = dep
	}
}

func depsToMap(deps []DependencyJSON) map[string]string {
	depsMap := make(map[string]string, len(deps))
	for _, dep := range deps {
		depsMap[dep.Name] = dep.Version
	}
	return depsMap
}

func processNPMPackage(packagePath string) error {
	packageJSONFile, err := os.ReadFile(packagePath)
	if err != nil {
		return err
	}

	var raw PackageJSONRaw
	if err = json.Unmarshal(packageJSONFile, &raw); err != nil {
		return err
	}

	var packageJSONDocument map[string]any
	if err = json.Unmarshal(packageJSONFile, &packageJSONDocument); err != nil {
		return err
	}

	packageJSON := PackageJSONDependencies{
		Dependencies:    mapToDeps(raw.Dependencies),
		DevDependencies: mapToDeps(raw.DevDependencies),
	}

	normalizeDependencyVersions(packageJSON.Dependencies)
	normalizeDependencyVersions(packageJSON.DevDependencies)

	fmt.Printf("Dependencies number : %v\n", len(packageJSON.Dependencies))
	updateDependencies(packageJSON.Dependencies)

	fmt.Printf("DevDependencies number : %v\n", len(packageJSON.DevDependencies))
	updateDependencies(packageJSON.DevDependencies)

	if raw.Dependencies != nil || packageJSONDocument["dependencies"] != nil {
		packageJSONDocument["dependencies"] = depsToMap(packageJSON.Dependencies)
	}

	if raw.DevDependencies != nil || packageJSONDocument["devDependencies"] != nil {
		packageJSONDocument["devDependencies"] = depsToMap(packageJSON.DevDependencies)
	}

	finalJSON, err := json.MarshalIndent(packageJSONDocument, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(packagePath, append(finalJSON, '\n'), 0644); err != nil {
		return err
	}

	fmt.Printf("Updated %s with final JSON\n", packagePath)
	return nil
}
