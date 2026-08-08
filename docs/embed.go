package docs

import "embed"

// OpenAPIFS contains the public API contract served by the web application.
//
//go:embed openapi.yaml
var OpenAPIFS embed.FS
