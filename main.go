package main

import (
	"fmt"
	"log"
	"os"
)

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
			if packagePath != "" {
				log.Panic("Multiple package.json files found in the current directory.")
			}

			packagePath = entry.Name()
			break
		}
	}

	if packagePath == "" {
		log.Panic("No package.json file found in the current directory.")
	}

	fmt.Println(packagePath)
}
