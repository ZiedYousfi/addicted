package main

import (
	"net/http"
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

	// Flags
	DryRun  bool
	Verbose bool
}

var Ctx = Context{
	ProjectType:     NotSupported,
	ProjectFilePath: "",
	HTTPClient:      &http.Client{Timeout: 5 * time.Second},
	DryRun:          false,
	Verbose:         false,
}
