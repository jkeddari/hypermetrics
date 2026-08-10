package assets

import "embed"

//go:embed css/* images/* js/*
var AssetsFS embed.FS
