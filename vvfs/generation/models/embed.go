//go:build embed_models
// +build embed_models

package models

// Build-tagged embedded support. When building with -tags embed_models,
// we include gguf/*.gguf into the binary and provide readEmbeddedModelBytes.

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed gguf/*.gguf
var embeddedGGUF embed.FS

// readEmbeddedModelBytes loads embedded model by filename from gguf/.
func readEmbeddedModelBytes(name string) ([]byte, error) {
	data, err := fs.ReadFile(embeddedGGUF, "gguf/"+name)
	if err != nil {
		return nil, fmt.Errorf("embedded model %s not found: %w", name, err)
	}
	return data, nil
}
