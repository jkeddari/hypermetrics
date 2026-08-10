package assets

import (
	"image/png"
	"testing"
)

func TestBrandImageDimensions(t *testing.T) {
	tests := []struct {
		path          string
		width, height int
	}{
		{path: "images/logo-512.png", width: 512, height: 512},
		{path: "images/apple-touch-icon.png", width: 180, height: 180},
		{path: "images/og-hypermetrics.png", width: 1200, height: 630},
	}
	for _, test := range tests {
		file, err := AssetsFS.Open(test.path)
		if err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if config.Width != test.width || config.Height != test.height {
			t.Fatalf("%s dimensions = %dx%d, want %dx%d", test.path, config.Width, config.Height, test.width, test.height)
		}
	}
}
