package models

// ModelType represents different types of GGUF models
type ModelType string

const (
	ModelTypeEmbedding ModelType = "embedding"
	ModelTypeChat      ModelType = "chat"
	ModelTypeVision    ModelType = "vision"
)
