package main

type TypeOfProject int

const (
	NotSupported TypeOfProject = iota
	NPM
	Cargo
)

type Context struct {
	ProjectType     TypeOfProject
	ProjectFilePath string
}

var Ctx = Context{
	ProjectType:     NPM,
	ProjectFilePath: "package.json",
}
