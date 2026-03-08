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
	fmt.Println(dir)

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

	fmt.Println(packagePath)
}
