package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
)

// Do not change it (maps in go can't be constants)
var FileForTypeOfProject = map[string]TypeOfProject{
	"package.json": NPM,
	"Cargo.toml":   Cargo,
}

func processCargoPackage(packagePath string) error {
	fmt.Printf("Cargo processing is not implemented yet for %s\n", packagePath)
	return nil
}

func processProjectFileByType(projectType TypeOfProject, projectFilePath string) error {
	switch projectType {
	case NPM:
		return processNPMPackage(projectFilePath)
	case Cargo:
		return processCargoPackage(projectFilePath)
	default:
		return fmt.Errorf("unsupported project type for %s", projectFilePath)
	}
}

func scanProjectFiles(entries []os.DirEntry, process func(TypeOfProject, string) error) (bool, error) {
	foundSupported := false

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		projectType, ok := FileForTypeOfProject[entry.Name()]
		if !ok {
			continue
		}

		foundSupported = true
		log.Infof("Found project file : %s", entry.Name())

		if err := process(projectType, entry.Name()); err != nil {
			return foundSupported, fmt.Errorf("error processing project file %s: %w", entry.Name(), err)
		}
	}

	return foundSupported, nil
}

func main() {
	log.SetLevel(log.WarnLevel)

	flag.BoolVar(&Ctx.DryRun, "dry-run", false, "Perform a dry run without making any changes")
	flag.BoolVar(&Ctx.DryRun, "d", false, "Perform a dry run without making any changes (shorthand)")
	flag.BoolVar(&Ctx.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&Ctx.Verbose, "v", false, "Enable verbose logging (shorthand)")

	flag.Parse()

	if Ctx.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Looking for project files in : %s", dir)

	entries, err := os.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	foundSupported, err := scanProjectFiles(entries, func(projectType TypeOfProject, projectFilePath string) error {
		Ctx.ProjectType = projectType
		Ctx.ProjectFilePath = projectFilePath
		return processProjectFileByType(Ctx.ProjectType, Ctx.ProjectFilePath)
	})

	if err != nil {
		log.Errorf("Project processing failed: %v", err)
		os.Exit(1)
	}

	if !foundSupported {
		log.Fatal("No supported project file found in the current directory.")
	}
}
