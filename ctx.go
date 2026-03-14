package main

import (
	"net/http"
	"time"

	"github.com/charmbracelet/log"
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
	Logger          *log.Logger

	// Flags
	DryRun  bool
	Verbose bool
}

var Ctx = Context{
	ProjectType:     NotSupported,
	ProjectFilePath: "",
	HTTPClient:      &http.Client{Timeout: 5 * time.Second},
	Logger:          log.Default(),
	DryRun:          false,
	Verbose:         false,
}
