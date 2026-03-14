package main

import (
	"io"
	"net/http"
	"os"
	"time"
)

type TypeOfProject int

const (
	NotSupported TypeOfProject = iota
	NPM
	Cargo
)

type Context struct {
	ProjectType     TypeOfProject
	ProjectFilePath string
	HTTPClient      *http.Client
	Output          io.Writer

	// Flags
	DryRun  bool
	Verbose bool
}

var Ctx = Context{
	ProjectType:     NotSupported,
	ProjectFilePath: "",
	HTTPClient:      &http.Client{Timeout: 5 * time.Second},
	Output:          os.Stdout,
	DryRun:          false,
	Verbose:         false,
}
