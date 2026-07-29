// Package tools carries the code generation directives for the repository.
//
// They live at the root rather than in cmd/go-via because `go generate` runs
// each directive with its own package's directory as the working directory,
// and these paths are relative to the repository root. The package is
// deliberately empty and is never imported.
package tools

//go:generate bash -c "go tool swag init -g cmd/go-via/main.go -o docs"
//go:generate bash -c "cd ui && npm ci && npm run build && cd .. && rm -rf webui/dist && cp -r ui/out webui/dist"
