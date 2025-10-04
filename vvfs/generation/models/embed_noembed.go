//go:build !embed_models
// +build !embed_models

package models

import "fmt"

// readEmbeddedModelBytes is disabled when not building with embed_models.
func readEmbeddedModelBytes(name string) ([]byte, error) {
	return nil, fmt.Errorf("embedded models disabled; use file-path loading")
}
