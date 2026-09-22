package assets

import "embed"

//go:embed templates
var FS embed.FS

//go:embed logo.png
var Logo []byte

//go:embed openapi.yaml
var OpenAPI []byte

//go:embed vendor/tailwind.css
var Tailwind string
