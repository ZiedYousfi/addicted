# addicted

`addicted` is a small Go CLI for updating project dependencies to their latest versions.

Right now, the project focuses on JavaScript projects that use `package.json`. It looks for a `package.json` file in the current directory, reads dependencies and devDependencies, fetches the latest versions from the npm registry, and writes the updated versions back to the file.

## Current scope

- Built in Go
- Works with npm-style `package.json` files
- Updates both `dependencies` and `devDependencies`

## Future plans

In the future, I want to expand `addicted` beyond npm and add support for other package managers and ecosystems, including:

- Cargo for Rust
- Maven for Java
- and potentially more package managers later

## Goal

The long-term goal is to make `addicted` a simple tool for keeping dependencies up to date across different languages and build systems.
