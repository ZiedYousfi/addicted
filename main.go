package main

import (
	"fmt"
	"github.com/charmbracelet/log"
	"os"
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

func processProjectFile() error {
	return processProjectFileByType(Ctx.ProjectType, Ctx.ProjectFilePath)
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

func scanProjectFiles(entries []os.DirEntry, process func(TypeOfProject, string) error) bool {
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
			log.Errorf("Error processing project file %s : %v", entry.Name(), err)
		}
	}

	return foundSupported
}

func main() {
	log.SetLevel(log.InfoLevel)

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Looking for project files in : %s", dir)

	entries, err := os.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	foundSupported := scanProjectFiles(entries, func(projectType TypeOfProject, projectFilePath string) error {
		Ctx.ProjectType = projectType
		Ctx.ProjectFilePath = projectFilePath
		return processProjectFile()
	})

	if !foundSupported {
		log.Fatal("No supported project file found in the current directory.")
	}
}
