package assets

import "embed"

//go:embed css/* js/*
var AssetsFS embed.FS
