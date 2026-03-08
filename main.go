package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("Hello, World!")

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(dir)

	packagePath := ""

	err = filepath.Walk(".",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.Contains(path, "package.json") {
				if packagePath != "" {
					log.Panicf("Multiple package.json files found. Previous: %s, Current: %s\n", packagePath, path)
				}
				packagePath = path
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}

	if packagePath == "" {
		log.Panic("No package.json file found in the current directory or its subdirectories.")
	}

	fmt.Println(packagePath)
}
