package main

import (
	"fmt"
	"log"
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
	switch Ctx.ProjectType {
	case NPM:
		return processNPMPackage(Ctx.ProjectFilePath)
	case Cargo:
		return processCargoPackage(Ctx.ProjectFilePath)
	default:
		return fmt.Errorf("unsupported project type for %s", Ctx.ProjectFilePath)
	}
}

func main() {
	fmt.Println("Hello, World!")

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Looking for project files in : %s\n", dir)

	entries, err := os.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		projectType, ok := FileForTypeOfProject[entry.Name()]
		if !ok {
			continue
		}

		fmt.Printf("Found project file : %s\n", entry.Name())

		Ctx.ProjectType = projectType
		Ctx.ProjectFilePath = entry.Name()

		if err := processProjectFile(); err != nil {
			log.Printf("Error processing project file %s: %v\n", entry.Name(), err)
		}
	}

	if Ctx.ProjectType == NotSupported {
		log.Panic("No supported project file found in the current directory.")
	}
}
