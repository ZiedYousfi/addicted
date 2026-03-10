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
}

var Ctx = Context{
	ProjectType:     NPM,
	ProjectFilePath: "package.json",
	HTTPClient:      &http.Client{Timeout: 5 * time.Second},
}
