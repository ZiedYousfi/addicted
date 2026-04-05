package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

type PackageJSONRaw struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type DependencyVersion struct {
	Raw       string
	Prefix    string
	Semver    Semver
	HasSemver bool
}

type DependencyJSON struct {
	Name    string
	Version DependencyVersion
}

type PackageJSONDependencies struct {
	Dependencies    []DependencyJSON
	DevDependencies []DependencyJSON
}

type DependencyUpdate struct {
	Name   string
	Before DependencyVersion
	After  DependencyVersion
}

func (v DependencyVersion) String() string {
	if v.HasSemver {
		return v.Prefix + v.Semver.String()
	}
	return v.Raw
}

func (v DependencyVersion) WithSemver(semver Semver) DependencyVersion {
	return DependencyVersion{
		Raw:       v.Prefix + semver.String(),
		Prefix:    v.Prefix,
		Semver:    semver,
		HasSemver: true,
	}
}

func parseDependencyVersion(value string) DependencyVersion {
	trimmed := strings.TrimSpace(value)
	prefix, coreVersion := extractDependencyVersionPrefix(trimmed)
	semver, err := ParseSemver(coreVersion)
	if err != nil {
		return DependencyVersion{Raw: trimmed}
	}

	return DependencyVersion{
		Raw:       trimmed,
		Prefix:    prefix,
		Semver:    semver,
		HasSemver: true,
	}
}

func extractDependencyVersionPrefix(value string) (string, string) {
	if value == "*" {
		return "", value
	}

	prefixes := []string{"<=", ">=", "^", "~", ">", "<", "=", "*"}
	for _, prefix := range prefixes {
		if rest, ok := strings.CutPrefix(value, prefix); ok {
			return prefix, rest
		}
	}
	return "", value
}

func mapToDeps(m map[string]string) []DependencyJSON {
	deps := make([]DependencyJSON, 0, len(m))
	for name, version := range m {
		deps = append(deps, DependencyJSON{Name: name, Version: parseDependencyVersion(version)})
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

func outputLogger() *log.Logger {
	if Ctx.Logger != nil {
		return Ctx.Logger
	}

	return log.Default()
}

func formatDependencyDiff(before DependencyVersion, after DependencyVersion) string {
	if before.HasSemver && after.HasSemver {
		if before.Prefix == "" && after.Prefix == "" {
			return before.Semver.Diff(after.Semver)
		}

		return fmt.Sprintf("%s -> %s (%s)", before.String(), after.String(), before.Semver.ChangeType(after.Semver))
	}

	return fmt.Sprintf("%s -> %s", before.String(), after.String())
}

func (update DependencyUpdate) String() string {
	return fmt.Sprintf("%s: %s", update.Name, formatDependencyDiff(update.Before, update.After))
}

func printDependencyUpdates(packagePath string, section string, updates []DependencyUpdate) {
	if len(updates) == 0 {
		return
	}

	action := "Updated"
	if Ctx.DryRun {
		action = "Would update"
	}

	logger := outputLogger()
	logger.Print(action + " " + section + " in " + packagePath + ":")
	for _, update := range updates {
		logger.Print("- " + update.String())
	}
}

func updateDependencies(deps []DependencyJSON) ([]DependencyUpdate, error) {
	updates := make([]DependencyUpdate, 0)

	for i, dep := range deps {
		log.Debugf("Dependency : %s, version : %s", dep.Name, dep.Version.String())
		if dep.Name == "" {
			log.Warnf("Dependency name is empty, skipping...")
			continue
		}

		latestVersionString, err := getNPMPackageLatestVersion(dep.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest version for %s: %w", dep.Name, err)
		}

		latestVersion := parseDependencyVersion(latestVersionString)
		log.Debugf("Latest version of %s : %s", dep.Name, latestVersion.String())

		changeType, shouldUpdate := classifyDependencyUpdate(dep.Version, latestVersion)
		if changeType == SemverChangeDowngrade {
			log.Warnf("Dependency %s current version %s is newer than registry latest %s, keeping current version", dep.Name, dep.Version.String(), latestVersion.String())
			continue
		}

		if !shouldUpdate {
			log.Debugf("Dependency %s won't update (%s)", dep.Name, changeType)
			continue
		}

		updatedVersion := mergeDependencyVersion(dep.Version, latestVersion)
		updates = append(updates, DependencyUpdate{Name: dep.Name, Before: dep.Version, After: updatedVersion})
		dep.Version = updatedVersion
		deps[i] = dep
	}
	return updates, nil
}

func classifyDependencyUpdate(currentVersion DependencyVersion, latestVersion DependencyVersion) (SemverChange, bool) {
	if currentVersion.HasSemver && latestVersion.HasSemver {
		changeType := currentVersion.Semver.ChangeType(latestVersion.Semver)
		if Ctx.PatchOnly && changeType != SemverChangePatch {
			return changeType, false
		}
		return changeType, changeType != SemverChangeNone && changeType != SemverChangeDowngrade
	}

	if currentVersion.String() == latestVersion.String() {
		return SemverChangeNone, false
	}

	return SemverChangeInvalid, true
}

func mergeDependencyVersion(currentVersion DependencyVersion, latestVersion DependencyVersion) DependencyVersion {
	if currentVersion.HasSemver && latestVersion.HasSemver {
		mergedSemver := latestVersion.Semver
		// Keep a revision suffix only when both versions refer to the same
		// semantic core; otherwise an old revision would be carried onto a new
		// base version like 1.2.4_1.
		if currentVersion.Semver.HasRevision && !latestVersion.Semver.HasRevision && semverCoreEqual(currentVersion.Semver, latestVersion.Semver) {
			mergedSemver.Revision = currentVersion.Semver.Revision
			mergedSemver.HasRevision = true
		}
		return currentVersion.WithSemver(mergedSemver)
	}

	return latestVersion
}

func semverCoreEqual(left Semver, right Semver) bool {
	return left.Major == right.Major && left.Minor == right.Minor && left.Patch == right.Patch
}

func depsToMap(deps []DependencyJSON) map[string]string {
	depsMap := make(map[string]string, len(deps))
	for _, dep := range deps {
		depsMap[dep.Name] = dep.Version.String()
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

	log.Infof("Dependencies number : %v", len(packageJSON.Dependencies))
	dependencyUpdates, err := updateDependencies(packageJSON.Dependencies)
	if err != nil {
		return fmt.Errorf("failed to update dependencies: %w", err)
	}

	log.Infof("DevDependencies number : %v", len(packageJSON.DevDependencies))
	devDependencyUpdates, err := updateDependencies(packageJSON.DevDependencies)
	if err != nil {
		return fmt.Errorf("failed to update devDependencies: %w", err)
	}

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

	if !Ctx.DryRun {
		if err := os.WriteFile(packagePath, append(finalJSON, '\n'), 0644); err != nil {
			return err
		}
		log.Infof("Updated %v with final JSON", packagePath)
	} else {
		log.Infof("Dry run enabled, not writing changes to %v", packagePath)
	}

	printDependencyUpdates(packagePath, "dependencies", dependencyUpdates)
	printDependencyUpdates(packagePath, "devDependencies", devDependencyUpdates)

	return nil
}
