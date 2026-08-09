package webui

import "embed"

// Files contains the production Vite bundle. Docker and CI build the frontend
// before compiling Go; the tracked .gitkeep keeps local Go tooling buildable.
//
//go:embed all:dist
var Files embed.FS
