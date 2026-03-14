package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withTestContext(t *testing.T, ctx Context) {
	t.Helper()
	previous := Ctx
	Ctx = ctx
	t.Cleanup(func() {
		Ctx = previous
	})
}

func newRegistryClient(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = targetURL.Scheme
			cloned.URL.Host = targetURL.Host
			cloned.Host = targetURL.Host
			return http.DefaultTransport.RoundTrip(cloned)
		}),
	}
}

func TestMapToDepsAndDepsToMapRoundTrip(t *testing.T) {
	depsMap := map[string]string{
		"react":  "^18.3.1",
		"vitest": "1.6.0",
	}

	deps := mapToDeps(depsMap)
	if len(deps) != len(depsMap) {
		t.Fatalf("expected %d dependencies, got %d", len(depsMap), len(deps))
	}

	for _, dep := range deps {
		if dep.Name != "react" {
			continue
		}
		if !dep.Version.HasSemver {
			t.Fatal("expected react version to parse as semver")
		}
		if dep.Version.Prefix != "^" {
			t.Fatalf("expected react prefix %q, got %q", "^", dep.Version.Prefix)
		}
	}

	roundTrip := depsToMap(deps)
	if !reflect.DeepEqual(roundTrip, depsMap) {
		t.Fatalf("expected round trip map %v, got %v", depsMap, roundTrip)
	}

	if got := mapToDeps(nil); len(got) != 0 {
		t.Fatalf("expected nil map to produce empty slice, got %v", got)
	}
}

func TestParseDependencyVersion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantString string
		wantPrefix string
		wantSemver bool
	}{
		{name: "caret", input: "^1.0.0", wantString: "^1.0.0", wantPrefix: "^", wantSemver: true},
		{name: "tilde revision", input: "~1.2.3_1", wantString: "~1.2.3_1", wantPrefix: "~", wantSemver: true},
		{name: "wildcard", input: "*", wantString: "*", wantPrefix: "", wantSemver: false},
		{name: "gte", input: ">=3.0.0", wantString: ">=3.0.0", wantPrefix: ">=", wantSemver: true},
		{name: "plain", input: "9.0.0", wantString: "9.0.0", wantPrefix: "", wantSemver: true},
		{name: "workspace", input: "workspace:*", wantString: "workspace:*", wantPrefix: "", wantSemver: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseDependencyVersion(test.input)
			if got.String() != test.wantString {
				t.Fatalf("parseDependencyVersion().String() = %q, want %q", got.String(), test.wantString)
			}
			if got.Prefix != test.wantPrefix {
				t.Fatalf("parseDependencyVersion().Prefix = %q, want %q", got.Prefix, test.wantPrefix)
			}
			if got.HasSemver != test.wantSemver {
				t.Fatalf("parseDependencyVersion().HasSemver = %v, want %v", got.HasSemver, test.wantSemver)
			}
		})
	}
}

func TestUpdateDependencies(t *testing.T) {
	client := newRegistryClient(t, func(w http.ResponseWriter, r *http.Request) {
		pkgPath := strings.TrimPrefix(r.URL.EscapedPath(), "/")
		pkgPath = strings.TrimSuffix(pkgPath, "/latest")
		pkgName, err := url.PathUnescape(pkgPath)
		if err != nil {
			t.Fatalf("unescape package path: %v", err)
		}

		switch pkgName {
		case "left-pad":
			_, _ = io.WriteString(w, `{"version":"1.3.0"}`)
		case "same-version":
			_, _ = io.WriteString(w, `{"version":"1.0.0"}`)
		case "underscore-style":
			_, _ = io.WriteString(w, `{"version":"1.2.4"}`)
		case "downgrade":
			_, _ = io.WriteString(w, `{"version":"1.9.9"}`)
		case "broken":
			http.Error(w, "boom", http.StatusBadGateway)
		default:
			t.Fatalf("unexpected package request: %s", pkgName)
		}
	})

	withTestContext(t, Context{HTTPClient: client})

	deps := []DependencyJSON{
		{Name: "left-pad", Version: parseDependencyVersion("1.1.0")},
		{Name: "", Version: parseDependencyVersion("2.0.0")},
		{Name: "broken", Version: parseDependencyVersion("3.0.0")},
	}

	err := updateDependencies(deps)
	if err == nil {
		t.Fatalf("expected error on broken package fetch, got nil")
	}
	if deps[0].Version.String() != "1.3.0" {
		t.Fatalf("expected successful dependency to update, got %q", deps[0].Version.String())
	}
	if deps[1].Version.String() != "2.0.0" {
		t.Fatalf("expected empty-name dependency to be skipped, got %q", deps[1].Version.String())
	}
	// The error should be due to the third dep (broken), so value should still be the old version
	if deps[2].Version.String() != "3.0.0" {
		t.Fatalf("expected failed lookup dependency to keep its version, got %q", deps[2].Version.String())
	}

}

func TestUpdateDependenciesSkipsNoopAndDowngrade(t *testing.T) {
	client := newRegistryClient(t, func(w http.ResponseWriter, r *http.Request) {
		pkgPath := strings.TrimPrefix(r.URL.EscapedPath(), "/")
		pkgPath = strings.TrimSuffix(pkgPath, "/latest")
		pkgName, err := url.PathUnescape(pkgPath)
		if err != nil {
			t.Fatalf("unescape package path: %v", err)
		}

		versions := map[string]string{
			"same-version":       "1.0.0",
			"underscore-style":   "1.2.4",
			"same-core-revision": "1.2.3",
			"downgrade":          "1.9.9",
			"prefixed":           "1.3.0",
		}

		version, ok := versions[pkgName]
		if !ok {
			t.Fatalf("unexpected package request: %s", pkgName)
		}

		_, _ = io.WriteString(w, `{"version":"`+version+`"}`)
	})

	withTestContext(t, Context{HTTPClient: client})

	deps := []DependencyJSON{
		{Name: "same-version", Version: parseDependencyVersion("1.0.0")},
		{Name: "underscore-style", Version: parseDependencyVersion("1.2.3_1")},
		{Name: "same-core-revision", Version: parseDependencyVersion("1.2.3_1")},
		{Name: "downgrade", Version: parseDependencyVersion("2.0.0")},
		{Name: "prefixed", Version: parseDependencyVersion("^1.2.0")},
	}

	if err := updateDependencies(deps); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if deps[0].Version.String() != "1.0.0" {
		t.Fatalf("expected same-version dependency to stay unchanged, got %q", deps[0].Version.String())
	}
	if deps[1].Version.String() != "1.2.4" {
		t.Fatalf("expected new base version to drop stale revision, got %q", deps[1].Version.String())
	}
	if deps[2].Version.String() != "1.2.3_1" {
		t.Fatalf("expected same semantic core to preserve revision, got %q", deps[2].Version.String())
	}
	if deps[3].Version.String() != "2.0.0" {
		t.Fatalf("expected downgrade dependency to stay unchanged, got %q", deps[3].Version.String())
	}
	if deps[4].Version.String() != "^1.3.0" {
		t.Fatalf("expected prefixed dependency to preserve prefix, got %q", deps[4].Version.String())
	}
}

func TestClassifyDependencyUpdate(t *testing.T) {
	tests := []struct {
		name       string
		current    DependencyVersion
		latest     DependencyVersion
		wantType   SemverChange
		wantUpdate bool
	}{
		{name: "patch", current: parseDependencyVersion("1.2.3"), latest: parseDependencyVersion("1.2.4"), wantType: SemverChangePatch, wantUpdate: true},
		{name: "none", current: parseDependencyVersion("1.2.3"), latest: parseDependencyVersion("1.2.3"), wantType: SemverChangeNone, wantUpdate: false},
		{name: "downgrade", current: parseDependencyVersion("2.0.0"), latest: parseDependencyVersion("1.9.9"), wantType: SemverChangeDowngrade, wantUpdate: false},
		{name: "revision", current: parseDependencyVersion("1.2.3"), latest: parseDependencyVersion("1.2.3_1"), wantType: SemverChangeRevision, wantUpdate: true},
		{name: "invalid fallback same", current: parseDependencyVersion("workspace:*"), latest: parseDependencyVersion("workspace:*"), wantType: SemverChangeNone, wantUpdate: false},
		{name: "invalid fallback different", current: parseDependencyVersion("workspace:*"), latest: parseDependencyVersion("2.0.0"), wantType: SemverChangeInvalid, wantUpdate: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotType, gotUpdate := classifyDependencyUpdate(test.current, test.latest)
			if gotType != test.wantType {
				t.Fatalf("classifyDependencyUpdate() type = %q, want %q", gotType, test.wantType)
			}
			if gotUpdate != test.wantUpdate {
				t.Fatalf("classifyDependencyUpdate() update = %v, want %v", gotUpdate, test.wantUpdate)
			}
		})
	}
}

func TestGetNPMPackageLatestVersion(t *testing.T) {
	t.Run("success with scoped package", func(t *testing.T) {
		var requestedPath string
		client := newRegistryClient(t, func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.EscapedPath()
			_, _ = io.WriteString(w, `{"version":"9.9.9"}`)
		})

		withTestContext(t, Context{HTTPClient: client})

		version, err := getNPMPackageLatestVersion("@scope/pkg")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if version != "9.9.9" {
			t.Fatalf("expected version 9.9.9, got %q", version)
		}
		if !strings.Contains(requestedPath, "%2F") {
			t.Fatalf("expected escaped path to contain encoded slash, got %q", requestedPath)
		}
		if !strings.HasSuffix(requestedPath, "/latest") {
			t.Fatalf("expected request path to end with /latest, got %q", requestedPath)
		}
	})

	t.Run("request error", func(t *testing.T) {
		withTestContext(t, Context{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		})}})

		_, err := getNPMPackageLatestVersion("react")
		if err == nil || !strings.Contains(err.Error(), "request error") {
			t.Fatalf("expected wrapped request error, got %v", err)
		}
	})

	t.Run("unexpected status", func(t *testing.T) {
		client := newRegistryClient(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusTeapot)
		})

		withTestContext(t, Context{HTTPClient: client})

		_, err := getNPMPackageLatestVersion("react")
		if err == nil || !strings.Contains(err.Error(), "unexpected status") {
			t.Fatalf("expected unexpected status error, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		client := newRegistryClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"version":`)
		})

		withTestContext(t, Context{HTTPClient: client})

		_, err := getNPMPackageLatestVersion("react")
		if err == nil || !strings.Contains(err.Error(), "JSON decode error") {
			t.Fatalf("expected JSON decode error, got %v", err)
		}
	})
}

func TestProcessNPMPackage(t *testing.T) {
	t.Run("updates dependencies and preserves document fields", func(t *testing.T) {
		tempDir := t.TempDir()
		packagePath := filepath.Join(tempDir, "package.json")
		input := `{
	  "name": "fixture",
	  "private": true,
	  "scripts": {
	    "test": "vitest"
	  },
	  "dependencies": {
	    "react": "^18.2.0",
	    "@scope/pkg": "~1.0.0"
	  },
	  "devDependencies": {
	    "vitest": ">=1.5.0"
	  }
	}
`
		if err := os.WriteFile(packagePath, []byte(input), 0644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}

		client := newRegistryClient(t, func(w http.ResponseWriter, r *http.Request) {
			pkgPath := strings.TrimPrefix(r.URL.EscapedPath(), "/")
			pkgPath = strings.TrimSuffix(pkgPath, "/latest")
			pkgName, err := url.PathUnescape(pkgPath)
			if err != nil {
				t.Fatalf("unescape package path: %v", err)
			}

			versions := map[string]string{
				"react":      "18.3.1",
				"@scope/pkg": "1.2.3",
				"vitest":     "1.6.0",
			}

			version, ok := versions[pkgName]
			if !ok {
				t.Fatalf("unexpected package request: %s", pkgName)
			}

			_, _ = io.WriteString(w, `{"version":"`+version+`"}`)
		})

		withTestContext(t, Context{HTTPClient: client})

		if err := processNPMPackage(packagePath); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		updated, err := os.ReadFile(packagePath)
		if err != nil {
			t.Fatalf("read updated package.json: %v", err)
		}
		if !strings.HasSuffix(string(updated), "\n") {
			t.Fatalf("expected output file to end with newline")
		}

		var document map[string]any
		if err := json.Unmarshal(updated, &document); err != nil {
			t.Fatalf("unmarshal updated package.json: %v", err)
		}

		deps, ok := document["dependencies"].(map[string]any)
		if !ok {
			t.Fatalf("expected dependencies object, got %T", document["dependencies"])
		}
		if deps["react"] != "^18.3.1" || deps["@scope/pkg"] != "~1.2.3" {
			t.Fatalf("unexpected dependency versions: %v", deps)
		}

		devDeps, ok := document["devDependencies"].(map[string]any)
		if !ok {
			t.Fatalf("expected devDependencies object, got %T", document["devDependencies"])
		}
		if devDeps["vitest"] != ">=1.6.0" {
			t.Fatalf("unexpected devDependency versions: %v", devDeps)
		}

		if document["name"] != "fixture" {
			t.Fatalf("expected name field to be preserved, got %v", document["name"])
		}
		if document["private"] != true {
			t.Fatalf("expected private field to be preserved, got %v", document["private"])
		}
	})

	t.Run("does not add missing dependency sections", func(t *testing.T) {
		tempDir := t.TempDir()
		packagePath := filepath.Join(tempDir, "package.json")
		input := `{
	  "name": "fixture",
	  "version": "1.0.0"
	}
`
		if err := os.WriteFile(packagePath, []byte(input), 0644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}

		withTestContext(t, Context{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("unexpected network call")
		})}})

		if err := processNPMPackage(packagePath); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		updated, err := os.ReadFile(packagePath)
		if err != nil {
			t.Fatalf("read updated package.json: %v", err)
		}

		var document map[string]any
		if err := json.Unmarshal(updated, &document); err != nil {
			t.Fatalf("unmarshal updated package.json: %v", err)
		}

		if _, ok := document["dependencies"]; ok {
			t.Fatalf("expected dependencies field to stay absent, got %v", document["dependencies"])
		}
		if _, ok := document["devDependencies"]; ok {
			t.Fatalf("expected devDependencies field to stay absent, got %v", document["devDependencies"])
		}
	})

	t.Run("returns parsing and file errors", func(t *testing.T) {
		tempDir := t.TempDir()
		invalidPackagePath := filepath.Join(tempDir, "package.json")
		if err := os.WriteFile(invalidPackagePath, []byte(`{"dependencies":`), 0644); err != nil {
			t.Fatalf("write invalid package.json: %v", err)
		}

		withTestContext(t, Context{HTTPClient: &http.Client{Timeout: time.Second}})

		if err := processNPMPackage(invalidPackagePath); err == nil {
			t.Fatal("expected invalid JSON to return an error")
		}

		missingPath := filepath.Join(tempDir, "missing-package.json")
		if err := processNPMPackage(missingPath); err == nil {
			t.Fatal("expected missing file to return an error")
		}
	})
}

func TestProcessProjectFileByType(t *testing.T) {
	withTestContext(t, Context{ProjectType: NotSupported, ProjectFilePath: "unknown.lock"})

	err := processProjectFileByType(Ctx.ProjectType, Ctx.ProjectFilePath)
	if err == nil {
		t.Fatal("expected unsupported project type to return an error")
	}
	if !strings.Contains(err.Error(), "unsupported project type") {
		t.Fatalf("expected unsupported project type error, got %v", err)
	}
}

func TestScanProjectFiles(t *testing.T) {
	t.Run("processes supported files and skips directories and unknown files", func(t *testing.T) {
		tempDir := t.TempDir()

		if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{}`), 0644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, "Cargo.toml"), []byte(`[package]`), 0644); err != nil {
			t.Fatalf("write Cargo.toml: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, "notes.txt"), []byte(`ignore me`), 0644); err != nil {
			t.Fatalf("write notes.txt: %v", err)
		}
		if err := os.Mkdir(filepath.Join(tempDir, "nested"), 0755); err != nil {
			t.Fatalf("create nested dir: %v", err)
		}

		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("read temp dir: %v", err)
		}

		processed := make(map[string]TypeOfProject)
		foundSupported, _ := scanProjectFiles(entries, func(projectType TypeOfProject, projectFilePath string) error {
			processed[projectFilePath] = projectType
			return nil
		})

		if !foundSupported {
			t.Fatal("expected supported project files to be found")
		}
		if len(processed) != 2 {
			t.Fatalf("expected 2 supported files to be processed, got %d", len(processed))
		}
		if processed["package.json"] != NPM {
			t.Fatalf("expected package.json to map to NPM, got %v", processed["package.json"])
		}
		if processed["Cargo.toml"] != Cargo {
			t.Fatalf("expected Cargo.toml to map to Cargo, got %v", processed["Cargo.toml"])
		}
		if _, ok := processed["notes.txt"]; ok {
			t.Fatal("expected unsupported file to be skipped")
		}
	})

	t.Run("returns false when nothing is supported", func(t *testing.T) {
		tempDir := t.TempDir()

		if err := os.WriteFile(filepath.Join(tempDir, "README.md"), []byte(`# docs`), 0644); err != nil {
			t.Fatalf("write README.md: %v", err)
		}
		if err := os.Mkdir(filepath.Join(tempDir, "src"), 0755); err != nil {
			t.Fatalf("create src dir: %v", err)
		}

		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("read temp dir: %v", err)
		}

		called := false
		foundSupported, _ := scanProjectFiles(entries, func(projectType TypeOfProject, projectFilePath string) error {
			called = true
			return nil
		})

		if foundSupported {
			t.Fatal("expected no supported project files to be found")
		}
		if called {
			t.Fatal("expected process callback not to be called")
		}
	})

	t.Run("continues after processing errors", func(t *testing.T) {
		tempDir := t.TempDir()

		if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{}`), 0644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, "Cargo.toml"), []byte(`[package]`), 0644); err != nil {
			t.Fatalf("write Cargo.toml: %v", err)
		}

		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("read temp dir: %v", err)
		}

		processed := make([]string, 0, 2)
		foundSupported, _ := scanProjectFiles(entries, func(projectType TypeOfProject, projectFilePath string) error {
			processed = append(processed, projectFilePath)
			if projectFilePath == "package.json" {
				return errors.New("boom")
			}
			return nil
		})

		if !foundSupported {
			t.Fatal("expected supported project files to be found")
		}
		if len(processed) != 2 {
			t.Fatalf("expected both supported files to be processed, got %v", processed)
		}
	})

	t.Run("works with synthetic dir entries", func(t *testing.T) {
		entries := []os.DirEntry{
			fakeDirEntry{name: "package.json"},
			fakeDirEntry{name: "nested", dir: true},
			fakeDirEntry{name: "pnpm-lock.yaml"},
		}

		processed := make([]string, 0, 1)
		foundSupported, _ := scanProjectFiles(entries, func(projectType TypeOfProject, projectFilePath string) error {
			processed = append(processed, projectFilePath)
			if projectType != NPM {
				t.Fatalf("expected NPM project type, got %v", projectType)
			}
			return nil
		})

		if !foundSupported {
			t.Fatal("expected supported file to be found")
		}
		if !reflect.DeepEqual(processed, []string{"package.json"}) {
			t.Fatalf("unexpected processed files: %v", processed)
		}
	})
}

type fakeDirEntry struct {
	name string
	dir  bool
}

func (f fakeDirEntry) Name() string {
	return f.name
}

func (f fakeDirEntry) IsDir() bool {
	return f.dir
}

func (f fakeDirEntry) Type() fs.FileMode {
	if f.dir {
		return fs.ModeDir
	}
	return 0
}

func (f fakeDirEntry) Info() (fs.FileInfo, error) {
	return nil, nil
}
