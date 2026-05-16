// Package web provee los assets del frontend embebidos en el binario.
package web

import "embed"

//go:embed all:dist
var FS embed.FS
